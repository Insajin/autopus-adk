package claude

import (
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

var claudeEmptyPrunePaths = []string{
	filepath.Join(".claude", "skills"),
	filepath.Join(".claude", "rules", "autopus"),
	filepath.Join(".claude", "agents", "autopus"),
	filepath.Join(".claude", "workflows"),
	filepath.Join(".claude", "hooks", "autopus"),
}

func (a *Adapter) validateEmptyPruneRoots() error {
	for _, path := range claudeEmptyPrunePaths {
		if err := adapter.RejectSymlinkComponents(a.root, path); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) pruneEmptyManagedDirs() error {
	if err := a.validateEmptyPruneRoots(); err != nil {
		return err
	}
	skillsRoot := filepath.Join(a.root, ".claude", "skills")
	if entries, err := os.ReadDir(skillsRoot); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				if err := removeEmptyDir(filepath.Join(skillsRoot, entry.Name())); err != nil {
					return err
				}
			}
		}
	}
	for _, path := range claudeEmptyPrunePaths[1:] {
		if err := removeEmptyDir(filepath.Join(a.root, path)); err != nil {
			return err
		}
	}
	return nil
}

func removeEmptyDir(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := removeEmptyDir(filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
	}
	entries, err = os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	return os.Remove(path)
}
