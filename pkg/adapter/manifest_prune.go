package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PruneManagedPaths removes compiler-owned stale artifacts from the workspace root.
func PruneManagedPaths(root string, entries []ManifestDiffEntry, backupDir *string) error {
	for _, entry := range entries {
		if entry.Action != ManifestActionPrune {
			continue
		}
		target, err := safePruneFilePath(root, entry.Path)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		if err := backupPrunedPath(root, entry, target, backupDir); err != nil {
			return err
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune managed path %s: %w", entry.Path, err)
		}
		if err := removeEmptyParents(root, filepath.Dir(target)); err != nil {
			return err
		}
	}
	return nil
}

func backupPrunedPath(root string, entry ManifestDiffEntry, target string, backupDir *string) error {
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read pruned path %s: %w", entry.Path, err)
	}
	if Checksum(string(data)) == entry.OldChecksum {
		return nil
	}

	if backupDir != nil && *backupDir == "" {
		dir, createErr := CreateBackupDir(root)
		if createErr != nil {
			return createErr
		}
		*backupDir = dir
	}
	if backupDir == nil || *backupDir == "" {
		return fmt.Errorf("backup dir unavailable for prune %s", entry.Path)
	}
	_, err = BackupFile(root, entry.Path, *backupDir)
	return err
}

func safePruneFilePath(root, relPath string) (string, error) {
	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." || filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe prune path %s", relPath)
	}

	target := filepath.Join(root, cleanRel)
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat prune path %s: %w", relPath, err)
	}

	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve prune path %s: %w", relPath, err)
	}
	normalizedRoot := realRoot + string(os.PathSeparator)
	if realTarget != realRoot && !strings.HasPrefix(realTarget, normalizedRoot) {
		return "", fmt.Errorf("refuse to prune path outside workspace: %s", relPath)
	}

	return target, nil
}

func removeEmptyParents(root, dir string) error {
	cleanRoot := filepath.Clean(root)
	current := filepath.Clean(dir)
	for current != cleanRoot && current != "." {
		entries, err := os.ReadDir(current)
		if err != nil {
			if os.IsNotExist(err) {
				current = filepath.Dir(current)
				continue
			}
			return fmt.Errorf("read prune parent %s: %w", current, err)
		}
		if len(entries) > 0 {
			return nil
		}
		if err := os.Remove(current); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty parent %s: %w", current, err)
		}
		current = filepath.Dir(current)
	}
	return nil
}
