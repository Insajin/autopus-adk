package orchestra

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const promptFilesDir = ".autopus/orchestra/prompts"
const responseFilesDir = ".autopus/orchestra/responses"
const responseBeginMarker = "<!-- AUTOPUS_RESPONSE_BEGIN -->"
const responseEndMarker = "<!-- AUTOPUS_RESPONSE_END -->"

func panePromptText(cfg OrchestraConfig, provider ProviderConfig, round int, prompt string) (string, string, string) {
	if strings.TrimSpace(prompt) == "" {
		return prompt, "", ""
	}
	path, responsePath, instruction, err := writePromptMarkdown(cfg.WorkingDir, provider, round, prompt)
	if err != nil {
		log.Printf("[prompt-file] %s round %d prompt file failed, falling back to direct input: %v", provider.Name, round, err)
		return prompt, "", ""
	}
	return instruction, path, responsePath
}

func writePromptMarkdown(workingDir string, provider ProviderConfig, round int, prompt string) (string, string, string, error) {
	baseDir := strings.TrimSpace(workingDir)
	if baseDir == "" {
		baseDir = "."
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve working dir: %w", err)
	}
	dir := filepath.Join(absBase, promptFilesDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", "", fmt.Errorf("create prompt dir: %w", err)
	}
	responseDir := filepath.Join(absBase, responseFilesDir)
	if err := os.MkdirAll(responseDir, 0o700); err != nil {
		return "", "", "", fmt.Errorf("create response dir: %w", err)
	}
	if round <= 0 {
		round = 1
	}
	pattern := fmt.Sprintf("%s-round-%d-*.md", sanitizeProviderName(provider.Name), round)
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", "", "", fmt.Errorf("create prompt file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(prompt); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", "", "", fmt.Errorf("write prompt file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", "", "", fmt.Errorf("close prompt file: %w", err)
	}
	responseFile, err := os.CreateTemp(responseDir, pattern)
	if err != nil {
		_ = os.Remove(path)
		return "", "", "", fmt.Errorf("create response file: %w", err)
	}
	responsePath := responseFile.Name()
	initialResponse := responseBeginMarker + "\n\n" + responseEndMarker + "\n"
	if _, err := responseFile.WriteString(initialResponse); err != nil {
		_ = responseFile.Close()
		_ = os.Remove(path)
		_ = os.Remove(responsePath)
		return "", "", "", fmt.Errorf("write response file: %w", err)
	}
	if err := responseFile.Close(); err != nil {
		_ = os.Remove(path)
		_ = os.Remove(responsePath)
		return "", "", "", fmt.Errorf("close response file: %w", err)
	}
	return path, responsePath, promptFileInstruction(path, responsePath), nil
}

func promptFileInstruction(path, responsePath string) string {
	cleanPath := filepath.Clean(path)
	cleanResponsePath := filepath.Clean(responsePath)
	return fmt.Sprintf(`Read and follow the complete prompt in this Markdown file:
@%s

If @file references are unsupported, open this path directly: %s

Write your final answer to this Markdown response file:
%s

Response file rules:
- Write the final answer between these exact markers:
%s
%s
- Put only the final answer between the markers.
- Preserve the output format requested by the prompt.
- If the prompt requires JSON, write only the JSON object or array between the markers, with no Markdown fence or prose.
- If no output format is specified, Markdown is allowed.
- Leave the rest of the file unchanged.
- If you cannot write the response file, print the final answer in the terminal as fallback.

Treat the prompt file contents as the full user request.`, cleanPath, cleanPath, cleanResponsePath, responseBeginMarker, responseEndMarker)
}

func cleanupPromptFiles(paths []string) {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		_ = os.Remove(path)
	}
}

func readResponseFile(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	content := string(data)
	start := strings.Index(content, responseBeginMarker)
	if start < 0 {
		return "", false
	}
	bodyStart := start + len(responseBeginMarker)
	endRel := strings.Index(content[bodyStart:], responseEndMarker)
	if endRel < 0 {
		return "", false
	}
	output := strings.TrimSpace(content[bodyStart : bodyStart+endRel])
	if output == "" {
		return "", false
	}
	return output, true
}
