package adapter

import (
	"errors"
	"fmt"
	"io/fs"
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

// resolvePruneRoot makes the workspace root absolute before symlinks are
// resolved. `--dir .` reaches here as a relative root, and EvalSymlinks keeps a
// relative path relative: `realRoot` became "." while the containment check
// below compared it against "./" — so every prune was refused as "outside
// workspace", and the Claude permission ledger read that same refusal as fatal.
// Absolute-first also gives removeEmptyParents a terminator it can actually
// reach; a relative root left the upward walk to stop only on a non-empty
// directory, and filepath.Dir("/") is "/".
//
// `exists` is false when the root is simply not there yet. Generate runs a
// permission-preimage read before the workspace is created, and nothing can be
// pruned from a directory that does not exist, so the callers below treat that
// as an empty prune set rather than an error.
func resolvePruneRoot(root string) (resolved string, exists bool, err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", false, fmt.Errorf("resolve workspace root: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve workspace root: %w", err)
	}
	return real, true, nil
}

func safePruneFilePath(root, relPath string) (string, error) {
	realRoot, exists, err := resolvePruneRoot(root)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." || filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe prune path %s", relPath)
	}

	target := filepath.Join(realRoot, cleanRel)
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat prune path %s: %w", relPath, err)
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
	cleanRoot, exists, err := resolvePruneRoot(root)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	current := filepath.Clean(dir)
	// `dir` and `root` can name the same tree in two different spellings -- on
	// macOS a temp root is /var/... while its resolved form is /private/var/...
	// An equality terminator misses that and the walk runs past the root, so the
	// loop below stops on containment instead and resolving here only widens how
	// often the two spellings agree.
	if resolved, resolveErr := filepath.EvalSymlinks(current); resolveErr == nil {
		current = resolved
	}
	for isStrictDescendant(cleanRoot, current) {
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

// isStrictDescendant reports whether path sits strictly below root. The root
// itself is not a descendant of itself, which is what keeps the workspace root
// out of every removal loop.
func isStrictDescendant(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
