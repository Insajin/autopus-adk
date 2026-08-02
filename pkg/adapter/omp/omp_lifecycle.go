package omp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
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

// cleanPruneRoots resolves ownership from the on-disk platform list. When that
// list is missing or unreadable, ownership of shared surfaces is unprovable, so
// Clean falls back to the surfaces omp owns unconditionally instead of assuming
// the maximal set.
//
// The existence check is load-bearing and easy to miss: config.Load returns a
// synthesized default (Platforms: ["claude-code"]) with a nil error when
// autopus.yaml is absent, and that default names neither codex nor opencode, so
// trusting it would put the shared skill surface back into the prune roots and
// delete a SKILL.md another platform now owns.
func (a *Adapter) cleanPruneRoots() []string {
	if !config.Exists(a.root) {
		return ompExclusivePruneRoots()
	}
	cfg, err := config.LoadPreview(a.root)
	if err != nil {
		return ompExclusivePruneRoots()
	}
	return PruneRoots(cfg)
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
