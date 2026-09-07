package companionmanifest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/linecount"
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
		counts, countErr := linecount.Source(path, data)
		if countErr != nil {
			return countErr
		}
		if counts.Counted > 300 {
			t.Errorf("%s has %d counted lines, want <= 300 (%d comment-only lines excluded)",
				path, counts.Counted, counts.CommentOnly)
		}
		return nil
	})
	if err != nil || checked == 0 {
		t.Fatalf("find release scripts: checked=%d: %v", checked, err)
	}
}
