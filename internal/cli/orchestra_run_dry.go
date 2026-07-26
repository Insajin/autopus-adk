package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// executeDryRun writes prompts to files without executing providers.
func executeDryRun(topic string, data orchestra.PromptData, providers []orchestra.ProviderConfig, rounds int) error {
	pb, err := orchestra.NewPromptBuilder()
	if err != nil {
		return fmt.Errorf("dry-run: %w", err)
	}

	r1Prompt, manifest, err := pb.BuildDebaterR1WithManifest(data)
	if err != nil {
		return fmt.Errorf("dry-run: r1 prompt: %w", err)
	}

	safeTopic := sanitizeFilename(topic)
	if safeTopic == "" {
		return fmt.Errorf("dry-run: topic does not produce a safe filename")
	}
	r1File := fmt.Sprintf("orchestra-r1-%s.md", safeTopic)
	if writeErr := writeNewPrivateFile(r1File, []byte(r1Prompt)); writeErr != nil {
		return fmt.Errorf("dry-run: write r1: %w", writeErr)
	}
	fmt.Printf("Round 1 prompt: %s\n", r1File)

	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: dry-run sidecar suffix is asserted by prompt manifest tooling.
	manifestFile := strings.TrimSuffix(r1File, ".md") + ".manifest.json"
	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("dry-run: manifest: %w", err)
	}
	if writeErr := writeNewPrivateFile(manifestFile, append(manifestBody, '\n')); writeErr != nil {
		return fmt.Errorf("dry-run: write manifest: %w", writeErr)
	}
	fmt.Printf("Round 1 prompt layer manifest: %s\n", manifestFile)

	schema := &orchestra.SchemaBuilder{}
	for _, role := range []string{"debater_r1", "debater_r2", "judge"} {
		path, cleanup, schemaErr := schema.WriteToFile(role)
		if schemaErr != nil {
			return fmt.Errorf("dry-run: schema %s: %w", role, schemaErr)
		}
		fmt.Printf("Schema (%s): %s\n", role, path)
		_ = cleanup // keep files in dry-run mode
	}

	fmt.Fprintf(os.Stderr, "Dry run complete. %d providers, %d rounds.\n", len(providers), rounds+1)
	return nil
}

// @AX:ANCHOR: [AUTO] exclusive private-file creation boundary for dry-run prompt and manifest artifacts
// @AX:REASON: [AUTO] symlink refusal, no-overwrite semantics, and 0600 permissions protect generated prompt evidence
func writeNewPrivateFile(path string, body []byte) error {
	if path == "" {
		return fmt.Errorf("empty output path")
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to overwrite symlink: %s", path)
		}
		return fmt.Errorf("refuse to overwrite existing file: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// sanitizeFilename replaces non-alphanumeric characters for safe filenames.
func sanitizeFilename(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '-')
		}
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result)
}
