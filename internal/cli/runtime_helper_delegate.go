package cli

import (
	"bytes"
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

func delegateRuntimeHelperStream(cmd *cobra.Command, helperArgs []string) error {
	return runRuntimeHelper(cmd, helperArgs, "", false)
}

func delegateRuntimeHelperJSON(cmd *cobra.Command, helperArgs []string) error {
	return runRuntimeHelper(cmd, helperArgs, cmd.CommandPath(), true)
}

func runRuntimeHelper(cmd *cobra.Command, helperArgs []string, commandPath string, rewriteJSON bool) error {
	program, err := resolveRuntimeHelperBinary()
	if err != nil {
		return err
	}

	execCmd := exec.CommandContext(cmd.Context(), program, helperArgs...) //nolint:gosec
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
		if path, err := exec.LookPath(override); err == nil {
			return path, nil
		}
		if isExecutableFile(override) {
			return override, nil
		}
		return "", fmt.Errorf("%s points to a missing or non-executable helper: %s", runtimeHelperOverrideEnv, override)
	}

	if path, err := exec.LookPath(runtimeHelperBinaryName); err == nil {
		return path, nil
	}

	if path := findSiblingRuntimeHelper(); path != "" {
		return path, nil
	}

	return "", fmt.Errorf(
		"desktop runtime helper not found; install/update the desktop app, stage the helper from ../autopus-desktop, or set %s",
		runtimeHelperOverrideEnv,
	)
}

func findSiblingRuntimeHelper() string {
	searchRoots := make([]string, 0, 8)

	if wd, err := os.Getwd(); err == nil {
		searchRoots = append(searchRoots, ancestorDirs(wd)...)
	}
	if exePath, err := os.Executable(); err == nil {
		searchRoots = append(searchRoots, ancestorDirs(filepath.Dir(exePath))...)
	}

	seen := make(map[string]struct{}, len(searchRoots))
	for _, root := range searchRoots {
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}

		patterns := []string{
			filepath.Join(root, "autopus-desktop", "src-tauri", "binaries", runtimeHelperBinaryName),
			filepath.Join(root, "autopus-desktop", "src-tauri", "binaries", runtimeHelperBinaryName+"*"),
		}
		if match := firstExecutableMatch(patterns); match != "" {
			return match
		}
	}

	return ""
}

func ancestorDirs(start string) []string {
	dirs := make([]string, 0, 8)
	current := filepath.Clean(start)
	for {
		dirs = append(dirs, current)
		parent := filepath.Dir(current)
		if parent == current {
			return dirs
		}
		current = parent
	}
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
