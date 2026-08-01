package cli

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
)

// TestPreviewPruneRoots_MatchClaudeApplyPath is the B1 symmetry invariant. The
// preview previously returned nil for claude-code, which isPruneEligible reads
// as "everything is eligible", so `auto update --plan` announced prunes the
// apply path could not perform. The manifest is rewritten either way, so those
// files became permanent orphans that no later run ever proposed again.
func TestPreviewPruneRoots_MatchClaudeApplyPath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, claude.PruneRoots(), previewPruneRoots("claude-code"),
		"preview must announce exactly the prunes the apply path performs")
}

// TestPreviewAndApplyComputeSamePruneSet exercises the invariant through the
// shared diff builder rather than through the root lists alone, so a change to
// isPruneEligible cannot break symmetry unnoticed.
func TestPreviewAndApplyComputeSamePruneSet(t *testing.T) {
	t.Parallel()

	relocated := []string{"lore-commit.md", "shell-portability.md", "worktree-safety.md"}
	oldManifest := &adapter.Manifest{Platform: "claude-code", Files: map[string]adapter.ManifestFile{
		".claude/rules/autopus/branding.md": {Checksum: "kept", Policy: adapter.OverwriteAlways},
		"CLAUDE.md":                         {Checksum: "outside", Policy: adapter.OverwriteMarker},
	}}
	for _, name := range relocated {
		oldManifest.Files[".claude/rules/autopus/"+name] = adapter.ManifestFile{
			Checksum: "stale", Policy: adapter.OverwriteAlways,
		}
	}

	newFiles := []adapter.FileMapping{{
		TargetPath:      filepath.Join(".claude", "rules", "autopus", "branding.md"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        "kept",
	}}

	previewPrune := prunePaths(adapter.BuildManifestDiff(oldManifest, newFiles, previewPruneRoots("claude-code")))
	applyPrune := prunePaths(adapter.BuildManifestDiff(oldManifest, newFiles, claude.PruneRoots()))

	assert.Equal(t, previewPrune, applyPrune, "preview and apply prune sets must match")
	assert.Equal(t, []string{
		".claude/rules/autopus/lore-commit.md",
		".claude/rules/autopus/shell-portability.md",
		".claude/rules/autopus/worktree-safety.md",
	}, applyPrune)
	assert.NotContains(t, applyPrune, "CLAUDE.md",
		"a managed file outside the compiler-owned roots is never pruned")
}

func prunePaths(diff adapter.ManifestDiff) []string {
	paths := make([]string, 0, len(diff.Prune))
	for _, entry := range diff.Prune {
		paths = append(paths, filepath.ToSlash(entry.Path))
	}
	sort.Strings(paths)
	return paths
}
