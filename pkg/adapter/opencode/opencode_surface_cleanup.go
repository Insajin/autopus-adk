package opencode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (a *Adapter) cleanupStaleSharedSkillDirs(oldManifest *adapter.Manifest, files []adapter.FileMapping) error {
	if oldManifest == nil {
		return nil
	}

	next := make(map[string]bool, len(files))
	for _, file := range files {
		next[file.TargetPath] = true
	}

	for path := range oldManifest.Files {
		if !isManagedOpenCodeSharedSkill(path) || next[path] {
			continue
		}
		target := filepath.Join(a.root, filepath.Dir(path))
		if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stale OpenCode shared skill 제거 실패 %s: %w", target, err)
		}
	}

	return nil
}

func isManagedOpenCodeSharedSkill(path string) bool {
	prefix := filepath.Join(".agents", "skills") + string(os.PathSeparator)
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return filepath.Base(path) == "SKILL.md"
}
