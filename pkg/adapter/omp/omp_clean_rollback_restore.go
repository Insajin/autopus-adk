package omp

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

func captureOMPCleanMissingParents(
	workspace *ompRootedWorkspace,
	path string,
) ([]ompCleanPreimage, error) {
	var reversed []ompCleanPreimage
	for parent := filepath.Dir(path); parent != "." && parent != ""; parent = filepath.Dir(parent) {
		info, err := workspace.lstat(parent)
		if errors.Is(err, fs.ErrNotExist) {
			reversed = append(reversed, ompCleanPreimage{path: parent, missing: true, directory: true})
			continue
		}
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("inspect clean backup parent %s", parent)
		}
		break
	}
	result := make([]ompCleanPreimage, len(reversed))
	for index := range reversed {
		result[len(reversed)-1-index] = reversed[index]
	}
	return result, nil
}

func pruneOMPCleanParents(
	workspace *ompRootedWorkspace,
	path string,
) ([]ompCleanPreimage, error) {
	var removed []ompCleanPreimage
	for clean := filepath.Clean(path); clean != "." && clean != ""; clean = filepath.Dir(clean) {
		entries, err := workspace.readDir(clean)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || len(entries) != 0 {
			return removed, err
		}
		info, err := workspace.lstat(clean)
		if err != nil {
			return removed, err
		}
		if err := workspace.remove(clean, false); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, ompCleanPreimage{path: clean, mode: info.Mode().Perm(), directory: true})
	}
	return removed, nil
}

func rollbackOMPClean(workspace *ompRootedWorkspace, applied []ompCleanPreimage) error {
	var rollbackErr error
	for index := len(applied) - 1; index >= 0; index-- {
		if err := restoreOMPCleanPreimage(workspace, applied[index]); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}

func restoreOMPCleanPreimage(workspace *ompRootedWorkspace, preimage ompCleanPreimage) error {
	if preimage.missing {
		err := workspace.remove(preimage.path, false)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if preimage.directory {
		directory, err := workspace.openDir(preimage.path, true, preimage.mode)
		if err != nil {
			return err
		}
		file, err := directory.Open(".")
		if err != nil {
			return errors.Join(err, directory.Close())
		}
		return errors.Join(file.Chmod(preimage.mode), file.Close(), directory.Close())
	}
	return workspace.atomicWrite(preimage.path, preimage.data, preimage.mode)
}
