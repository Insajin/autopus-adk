package omp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// Validate checks the validity of installed OMP files.
func (a *Adapter) Validate(ctx context.Context) ([]adapter.ValidationError, error) {
	return a.validateInstalledSurface(ctx)
}

// Clean removes files created by this adapter.
func (a *Adapter) Clean(ctx context.Context) error {
	_, err := a.CleanWithReceipt(ctx)
	return err
}

// cleanPruneRoots includes current native roots and legacy roots. Clean still
// selects only paths recorded in the OMP manifest, so foreign files are safe.
func (a *Adapter) cleanPruneRoots() []string {
	return ompExclusivePruneRoots()
}

func isPruneEligible(path string, roots []string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return false
	}
	if len(roots) == 0 {
		return true
	}
	for _, root := range roots {
		normalizedRoot := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(root)), "/")
		if clean == normalizedRoot || strings.HasPrefix(clean, normalizedRoot+"/") {
			return true
		}
	}
	return false
}

func removeEmptyParents(root, dir string) error {
	cleanRoot := filepath.Clean(root)
	current := filepath.Clean(dir)
	for current != cleanRoot && current != "." {
		// The same parent-symlink hazard applies to directory removal, so stop
		// the walk as soon as a level cannot be proven to live in the workspace.
		if !withinWorkspace(cleanRoot, current) {
			return nil
		}
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

// withinWorkspace reports whether candidate resolves inside root once every
// symlink on both paths is followed. A directory that resolves elsewhere is not
// ours to remove.
func withinWorkspace(root, candidate string) bool {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// A path that no longer exists cannot be removed through a link either.
		return os.IsNotExist(err)
	}
	return realCandidate == realRoot ||
		strings.HasPrefix(realCandidate, realRoot+string(os.PathSeparator))
}
