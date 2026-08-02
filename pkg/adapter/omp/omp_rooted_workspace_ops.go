package omp

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func (workspace *ompRootedWorkspace) copyFile(source, target string, perm os.FileMode) error {
	data, _, err := workspace.readFile(source, 0)
	if err != nil {
		return err
	}
	return workspace.atomicWrite(target, data, perm)
}

func (workspace *ompRootedWorkspace) removeEmptyParents(path string) error {
	clean := filepath.Clean(path)
	for clean != "." && clean != "" {
		entries, err := workspace.readDir(clean)
		if errors.Is(err, fs.ErrNotExist) {
			clean = filepath.Dir(clean)
			continue
		}
		if err != nil || len(entries) != 0 {
			return err
		}
		if err := workspace.remove(clean, false); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		clean = filepath.Dir(clean)
	}
	return nil
}
