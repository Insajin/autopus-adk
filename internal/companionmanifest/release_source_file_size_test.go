package companionmanifest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseScripts_SourceFilesStayBelowLimit(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "scripts", "companion-release")
	checked := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".sh" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		checked++
		if lines := strings.Count(string(data), "\n") + 1; lines > 300 {
			t.Errorf("%s has %d lines, want <= 300", path, lines)
		}
		return nil
	})
	if err != nil || checked == 0 {
		t.Fatalf("find release scripts: checked=%d: %v", checked, err)
	}
}
