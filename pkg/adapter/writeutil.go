package adapter

import (
	"bytes"
	"os"
)

// WriteFileIfChanged writes content to path only when the existing file content
// differs. This prevents unnecessary file mtime updates that trigger settings
// reload in host tools (e.g., Claude Code permission re-evaluation).
func WriteFileIfChanged(path string, content []byte, perm os.FileMode) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil // no change — skip write to preserve file mtime
	}
	return os.WriteFile(path, content, perm)
}
