package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	runtimeHelperBinaryName  = "autopus-desktop-runtime"
	runtimeHelperOverrideEnv = "AUTOPUS_DESKTOP_RUNTIME_HELPER"
)

var errRuntimeHelperNotFound = errors.New("desktop runtime helper not found")
var resolveRuntimeHelper = resolveRuntimeHelperBinary
var runtimeHelperPackagedPatterns = defaultRuntimeHelperPackagedPatterns

func delegateRuntimeHelperStream(cmd *cobra.Command, helperArgs []string) error {
	return runRuntimeHelper(cmd, helperArgs, "", false)
}

func delegateRuntimeHelperJSON(cmd *cobra.Command, helperArgs []string) error {
	return runRuntimeHelper(cmd, helperArgs, cmd.CommandPath(), true)
}

func runRuntimeHelper(cmd *cobra.Command, helperArgs []string, commandPath string, rewriteJSON bool) error {
	program, err := resolveRuntimeHelper()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	execCmd := exec.CommandContext(ctx, program, helperArgs...) //nolint:gosec
	execCmd.Stdin = cmd.InOrStdin()
	execCmd.Stderr = cmd.ErrOrStderr()

	var stdout bytes.Buffer
	if rewriteJSON {
		execCmd.Stdout = &stdout
	} else {
		execCmd.Stdout = cmd.OutOrStdout()
	}

	err = execCmd.Run()
	if rewriteJSON && stdout.Len() > 0 {
		payload := stdout.Bytes()
		if commandPath != "" {
			rewritten, rewriteErr := rewriteRuntimeHelperEnvelope(payload, commandPath)
			if rewriteErr == nil {
				payload = rewritten
			}
		}
		if _, writeErr := cmd.OutOrStdout().Write(payload); writeErr != nil {
			return writeErr
		}
	}
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
		return nil
	}

	return fmt.Errorf("run desktop runtime helper: %w", err)
}

func resolveRuntimeHelperBinary() (string, error) {
	override := strings.TrimSpace(os.Getenv(runtimeHelperOverrideEnv))
	if override != "" {
		return resolveRuntimeHelperOverride(override)
	}

	// @AX:NOTE [AUTO]: Avoid PATH/cwd discovery because delegated auth import streams credential JSON to the helper process.
	if path := findPackagedRuntimeHelper(); path != "" {
		return path, nil
	}

	return "", fmt.Errorf(
		"%w; install/update the desktop app, or set %s to an absolute helper path",
		errRuntimeHelperNotFound,
		runtimeHelperOverrideEnv,
	)
}

func resolveRuntimeHelperOverride(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s must be an absolute helper path: %s", runtimeHelperOverrideEnv, path)
	}
	if isExecutableFile(path) {
		return filepath.Clean(path), nil
	}
	return "", fmt.Errorf("%s points to a missing or non-executable helper: %s", runtimeHelperOverrideEnv, path)
}

func findPackagedRuntimeHelper() string {
	return firstExecutableMatch(runtimeHelperPackagedPatterns())
}

func defaultRuntimeHelperPackagedPatterns() []string {
	patterns := make([]string, 0, 24)

	if exePath, err := os.Executable(); err == nil {
		patterns = append(patterns, packagedRuntimeHelperPatternsFrom(filepath.Dir(exePath))...)
	}
	patterns = append(patterns, macOSRuntimeHelperPatterns()...)

	return patterns
}

func packagedRuntimeHelperPatternsFrom(exeDir string) []string {
	contentsDir := filepath.Dir(exeDir)
	return []string{
		filepath.Join(exeDir, runtimeHelperBinaryName),
		filepath.Join(exeDir, runtimeHelperBinaryName+"*"),
		filepath.Join(contentsDir, "Resources", runtimeHelperBinaryName),
		filepath.Join(contentsDir, "Resources", runtimeHelperBinaryName+"*"),
		filepath.Join(contentsDir, "Resources", "binaries", runtimeHelperBinaryName),
		filepath.Join(contentsDir, "Resources", "binaries", runtimeHelperBinaryName+"*"),
	}
}

func macOSRuntimeHelperPatterns() []string {
	appNames := []string{"Autopus.app", "Autopus Desktop.app"}
	patterns := make([]string, 0, len(appNames)*6)

	for _, appName := range appNames {
		contentsDir := filepath.Join(string(filepath.Separator), "Applications", appName, "Contents")
		for _, dir := range []string{
			filepath.Join(contentsDir, "MacOS"),
			filepath.Join(contentsDir, "Resources"),
			filepath.Join(contentsDir, "Resources", "binaries"),
		} {
			patterns = append(patterns,
				filepath.Join(dir, runtimeHelperBinaryName),
				filepath.Join(dir, runtimeHelperBinaryName+"*"),
			)
		}
	}
	return patterns
}

func firstExecutableMatch(patterns []string) string {
	for _, pattern := range patterns {
		if !strings.Contains(pattern, "*") {
			if isExecutableFile(pattern) {
				return pattern
			}
			continue
		}

		matches, globErr := filepath.Glob(pattern)
		if globErr != nil {
			continue
		}
		sort.Strings(matches)
		for _, match := range matches {
			if isExecutableFile(match) {
				return match
			}
		}
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func appendStringFlag(args []string, name, value string, include bool) []string {
	if !include {
		return args
	}
	return append(args, "--"+name, value)
}

func appendBoolFlag(args []string, name string, include bool) []string {
	if !include {
		return args
	}
	return append(args, "--"+name)
}

func appendDurationFlag(args []string, name string, value time.Duration, include bool) []string {
	if !include {
		return args
	}
	return append(args, "--"+name, value.String())
}

func rewriteRuntimeHelperEnvelope(data []byte, commandPath string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	payload["command"] = commandPath

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeString(ioWriter io.Writer, text string) error {
	_, err := io.WriteString(ioWriter, text)
	return err
}
