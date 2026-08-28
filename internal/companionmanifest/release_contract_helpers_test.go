package companionmanifest

import (
	"os"
	"path/filepath"
	"testing"
)

// releaseWorkflowLineAt returns the source line containing offset.
func releaseWorkflowLineAt(source string, offset int) string {
	start := offset
	for start > 0 && source[start-1] != '\n' {
		start--
	}
	end := offset
	for end < len(source) && source[end] != '\n' {
		end++
	}
	return source[start:end]
}

func readReleaseFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
