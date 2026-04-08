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
