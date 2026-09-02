package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// retiredCodexSurfaces are the orphan shapes a workspace installed before the
// native layout still carries: the flat skill layout, the whole retired prompt
// directory, and the retired rule tree.
var retiredCodexSurfaces = []string{
	".codex/skills/retired-surface.md",
	".codex/prompts/auto.md",
	".codex/rules/autopus/context7-docs.md",
}

func writeCodexFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(body), 0o644))
	return abs
}

// TestUpdate_RemovesRetiredSurfaceWithoutManifest is the fresh-clone regression.
//
// `.autopus/codex-manifest.json` is gitignored while the generated files are
// committed, so a cloned workspace carries orphans with no ownership record.
// The manifest-only prune could not see them — BuildManifestDiff iterates
// old.Files, and old is nil here — and the update then rewrote the manifest
// without them, so no later update could ever propose the prune either. The
// orphan was permanent.
func TestUpdate_RemovesRetiredSurfaceWithoutManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, rel := range retiredCodexSurfaces {
		writeCodexFile(t, dir, rel, "retired")
	}

	require.NoFileExists(t, filepath.Join(dir, ".autopus", "codex-manifest.json"),
		"the reproduction requires the no-manifest arm")

	_, err := NewWithRoot(dir).Update(context.Background(), config.DefaultFullConfig("orphan-project"))
	require.NoError(t, err)

	for _, rel := range retiredCodexSurfaces {
		assert.NoFileExists(t, filepath.Join(dir, filepath.FromSlash(rel)),
			"%s must not survive an update that had no manifest to prune from", rel)
	}
	// No empty husk may remain where a retired surface used to live: the
	// transaction's removeEmptyParents walks up from each deleted file.
	assert.NoDirExists(t, filepath.Join(dir, ".codex", "prompts"),
		"the retired prompt directory itself must go, not just its files")
	assert.NoDirExists(t, filepath.Join(dir, ".codex", "rules"),
		"the retired rule tree must not survive as empty directories")
}

// TestUpdate_PreservesUserFilesInsidePruneRoots is the safety half. Every path
// asserted here lives inside a PruneRoots subtree, so only the closed detector
// set keeps them alive: a rule of the "delete whatever the manifest does not
// claim" shape would destroy all of them on a fresh clone.
func TestUpdate_PreservesUserFilesInsidePruneRoots(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	survivors := map[string]string{
		// Native/user skill layout: a directory child, not a flat `*.md`.
		".codex/skills/my-own-skill/SKILL.md": "user skill",
		// Not markdown, so outside the retired flat-skill shape.
		".codex/skills/notes.txt": "user notes",
	}
	for rel, body := range survivors {
		writeCodexFile(t, dir, rel, body)
	}

	_, err := NewWithRoot(dir).Update(context.Background(), config.DefaultFullConfig("orphan-project"))
	require.NoError(t, err)

	for rel, body := range survivors {
		data, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		require.NoError(t, readErr, "%s must survive: it is not in the retired set", rel)
		assert.Equal(t, body, string(data), rel)
	}
}

// TestObsoleteCodexSurfacePaths_SkipsSharedSkillsOwnedByOpenCode pins the
// platform gate: `.agents/skills/<workflow>` is a shared surface, and when
// OpenCode is installed it owns those directories.
func TestObsoleteCodexSurfacePaths_SkipsSharedSkillsOwnedByOpenCode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCodexFile(t, dir, ".agents/skills/auto/SKILL.md", "shared")

	assert.Contains(t, obsoleteCodexSurfacePaths(dir, false), ".agents/skills/auto")
	assert.NotContains(t, obsoleteCodexSurfacePaths(dir, true), ".agents/skills/auto",
		"OpenCode owns the shared skill tree; the Codex adapter must not delete it")
}

// TestUpdate_PreservesSharedSkillsOnFreshCloneWithOpenCodeConfigured guards the
// hazard the manifest-independent prune creates. `.autopus/opencode-manifest.json`
// is gitignored, so on a fresh clone the manifest-only ownership check answers
// "OpenCode is not here" and the prune would delete `.agents/skills/<workflow>`,
// a live surface opencode_skills.go emits. The configured platform list is the
// manifest-independent half of the gate.
func TestUpdate_PreservesSharedSkillsOnFreshCloneWithOpenCodeConfigured(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	shared := writeCodexFile(t, dir, ".agents/skills/auto/SKILL.md", "opencode shared skill")
	require.NoFileExists(t, filepath.Join(dir, ".autopus", "opencode-manifest.json"),
		"the hazard only appears when the OpenCode manifest did not survive the clone")

	cfg := config.DefaultFullConfig("mixed-project")
	cfg.Platforms = []string{"codex", "opencode"}
	require.False(t, NewWithRoot(dir).openCodeOwnsRootDoc(),
		"the manifest-only check must be the false negative this test defends against")

	_, err := NewWithRoot(dir).Update(context.Background(), cfg)
	require.NoError(t, err)

	data, readErr := os.ReadFile(shared)
	require.NoError(t, readErr, "a live OpenCode surface must survive a Codex update")
	assert.Equal(t, "opencode shared skill", string(data))
}

// TestValidateObsoleteCodexSurface_ReportsExactlyTheDetectorSet pins the
// invariant that makes the fix safe to reason about: what `auto doctor` calls
// obsolete is exactly what the update removes, because both read one detector.
func TestValidateObsoleteCodexSurface_ReportsExactlyTheDetectorSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, rel := range retiredCodexSurfaces {
		writeCodexFile(t, dir, rel, "retired")
	}
	writeCodexFile(t, dir, ".codex/skills/my-own-skill/SKILL.md", "user skill")

	var errs []adapter.ValidationError
	validateObsoleteCodexSurface(dir, false, &errs)

	reported := make([]string, 0, len(errs))
	for _, item := range errs {
		assert.Equal(t, "error", item.Level, item.File)
		assert.Equal(t, "obsolete Codex managed surface가 남아 있음", item.Message, item.File)
		reported = append(reported, item.File)
	}
	assert.Equal(t, obsoleteCodexSurfacePaths(dir, false), reported)
	assert.Equal(t, []string{
		".codex/prompts",
		".codex/rules/autopus/context7-docs.md",
		".codex/skills/retired-surface.md",
	}, reported)
}
