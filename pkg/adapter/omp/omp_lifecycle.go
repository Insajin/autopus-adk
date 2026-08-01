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
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

	checks := []struct {
		path    string
		message string
		level   string
	}{
		{configFile, ".omp/config.yml이 없음", "error"},
		{filepath.Join(".omp", "agents"), "omp agent 디렉터리가 없음", "error"},
		{filepath.Join(".agents", "rules", "autopus"), "omp rule 디렉터리가 없음", "error"},
	}
	for _, check := range checks {
		if _, statErr := os.Stat(filepath.Join(a.root, check.path)); os.IsNotExist(statErr) {
			errs = append(errs, adapter.ValidationError{File: check.path, Message: check.message, Level: check.level})
		}
	}

	return errs, nil
}

// Clean removes files created by this adapter.
func (a *Adapter) Clean(_ context.Context) error {
	manifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return err
	}
	if manifest == nil {
		return nil
	}

	roots := a.cleanPruneRoots()
	var backupDir string

	for path, prev := range manifest.Files {
		if strings.HasPrefix(path, filepath.Join(".git", "hooks")) {
			continue
		}

		if !isPruneEligible(path, roots) {
			continue
		}

		// Resolve the entry against the workspace before touching it. os.Remove
		// follows PARENT-directory symlinks, so a link planted above a managed
		// file would delete a file outside the workspace — and `auto platform
		// remove omp` discards Clean's error, so that deletion would be silent.
		// An entry that cannot be proven to live inside the workspace is skipped
		// whole: leaving a file behind beats unlinking through an unverified path.
		target, safeErr := adapter.SafePruneFilePath(a.root, path)
		if safeErr != nil || target == "" {
			continue
		}

		// A marker-policy file holds user content around the managed section.
		// Its manifest checksum covers the merged document, so an untouched
		// user file matches and would be deleted without a backup. Strip the
		// managed section and keep whatever the user owns (REQ-007, REQ-017).
		if prev.Policy == adapter.OverwriteMarker {
			if stripErr := a.stripManagedSection(path, target); stripErr != nil {
				return stripErr
			}
			continue
		}

		// Containment proves the target is inside the workspace, which a parent
		// symlink pointing at another in-workspace directory also satisfies. That
		// link still breaks the identity between the manifest path and the file
		// on disk, so the entry is skipped WHOLE rather than backed up and
		// removed separately: gating only the backup deleted a checksum-mismatched
		// file with no copy kept, and Clean's error is discarded by
		// `auto platform remove omp`, so the loss was silent.
		if adapter.RejectSymlinkComponents(a.root, path) != nil {
			continue
		}

		data, readErr := os.ReadFile(target)
		if readErr == nil && adapter.Checksum(string(data)) != prev.Checksum {
			if backupDir == "" {
				dir, createErr := adapter.CreateBackupDir(a.root)
				if createErr != nil {
					return createErr
				}
				backupDir = dir
			}
			_, _ = adapter.BackupFile(a.root, path, backupDir)
		}

		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("%s 제거 실패: %w", path, removeErr)
		}

		_ = removeEmptyParents(a.root, filepath.Dir(target))
	}

	// The manifest removal needs the same containment as the entries above: a
	// symlinked .autopus/ would otherwise resolve this path outside the
	// workspace and delete another project's manifest.
	manifestRel := filepath.Join(".autopus", adapterName+"-manifest.json")
	if manifestTarget, safeErr := adapter.SafePruneFilePath(a.root, manifestRel); safeErr == nil && manifestTarget != "" {
		_ = os.Remove(manifestTarget)
	}
	return nil
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
	cfg, err := config.Load(a.root)
	if err != nil {
		return ompExclusivePruneRoots()
	}
	return PruneRoots(cfg)
}

// stripManagedSection removes the managed marker block and rewrites the rest.
// A file left holding nothing but whitespace carried no user content, so it is
// removed rather than left behind as an empty husk.
func (a *Adapter) stripManagedSection(relPath, target string) error {
	if err := adapter.RejectSymlinkComponents(a.root, relPath); err != nil {
		return nil
	}
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s 읽기 실패: %w", relPath, err)
	}

	// The regex is blind to YAML structure, so run the same scalar guard the
	// Generate path uses. Without it `auto platform remove omp` stripped through
	// a marker the user had pasted into a block scalar and lost that value — with
	// no backup on this path and the caller discarding the error.
	if err := rejectMarkerInsideScalar(string(data)); err != nil {
		return nil
	}

	remainder := strings.TrimSpace(markerReYml.ReplaceAllString(string(data), ""))
	if remainder == "" {
		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("%s 제거 실패: %w", relPath, removeErr)
		}
		_ = removeEmptyParents(a.root, filepath.Dir(target))
		return nil
	}

	if writeErr := os.WriteFile(target, []byte(remainder+"\n"), 0644); writeErr != nil {
		return fmt.Errorf("%s 쓰기 실패: %w", relPath, writeErr)
	}
	return nil
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
