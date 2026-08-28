package omp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// hardenLegacyOMPFilesForUpdate narrows the read-only modes written by
// pre-v0.50.109 releases before update admits their manifest or sensitive
// project config. Clean remains fail-closed and never performs this migration.
func hardenLegacyOMPFilesForUpdate(workspace *ompRootedWorkspace, platform string) error {
	paths := []string{
		filepath.Join(".autopus", platform+"-manifest.json"),
		configFile,
	}
	for _, path := range paths {
		if err := hardenLegacyOMPFileForUpdate(workspace, path); err != nil {
			return err
		}
	}
	return nil
}

func hardenLegacyOMPFileForUpdate(
	workspace *ompRootedWorkspace,
	path string,
) (returnErr error) {
	file, before, err := workspace.openRegular(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, file.Close()) }()

	perm := before.Mode().Perm()
	if perm == 0o600 {
		return nil
	}
	if perm&0o600 != 0o600 || perm&0o022 != 0 || perm&^os.FileMode(0o644) != 0 {
		return fmt.Errorf("OMP path %s must have mode 0600", path)
	}
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("harden legacy OMP path %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync hardened OMP path %s: %w", path, err)
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || after.Mode().Perm() != 0o600 {
		return fmt.Errorf("OMP path %s changed while hardening legacy permissions", path)
	}
	return nil
}
