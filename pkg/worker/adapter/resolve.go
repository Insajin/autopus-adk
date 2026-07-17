// Package adapter — resolve: CLI binary path resolution with well-known fallbacks.
//
// LaunchD and other daemon environments have restricted PATH.
// This resolver searches well-known installation locations when exec.LookPath fails.
package adapter

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// wellKnownDirs lists directories where CLI tools are commonly installed.
// Checked in order when the binary is not found in PATH.
var wellKnownDirs = func() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
	}
	if runtime.GOOS == "darwin" {
		dirs = append(dirs,
			"/Applications/cmux.app/Contents/Resources/bin",
		)
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".npm-global", "bin"),
			filepath.Join(home, ".cargo", "bin"),
		)
	}
	return dirs
}()

// ResolveBinary finds the full path to a CLI binary.
// First checks exec.LookPath (respects PATH), then searches well-known directories.
func ResolveBinary(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, dir := range wellKnownDirs {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	// Fallback: return bare name, let exec.Command fail with a clear error.
	return name
}

// EnvironWithToolPath appends well-known CLI install directories to PATH,
// placed after the inherited PATH so identically named binaries resolve from
// the inherited PATH first. GUI-launched desktop apps often inherit a
// restricted PATH, while npm-based provider shims such as Codex resolve node
// through /usr/bin/env; appending keeps those tools discoverable without
// letting a user-writable well-known directory shadow the inherited PATH.
func EnvironWithToolPath(env []string) []string {
	currentPath := envValue(env, "PATH")
	var parts []string
	if currentPath != "" {
		parts = append(parts, strings.Split(currentPath, string(os.PathListSeparator))...)
	}
	// Trade-off: a binary present in both the inherited PATH and a
	// well-known directory now always resolves from the inherited PATH.
	parts = append(parts, wellKnownDirs...)

	seen := make(map[string]bool, len(parts))
	merged := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		merged = append(merged, part)
	}

	next := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, "PATH=") {
			continue
		}
		next = append(next, item)
	}
	next = append(next, "PATH="+strings.Join(merged, string(os.PathListSeparator)))
	return next
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}
