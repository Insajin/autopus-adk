package config

// Issue #187: the review gate's revision budget must tell an omitted
// max_revisions apart from an explicitly configured zero. With a plain int both
// decoded to 0 and the consumer re-applied its default, so "run one round" was
// unexpressible.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewGateConf_ResolveMaxRevisions(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 3, ReviewGateConf{}.ResolveMaxRevisions(3),
		"an omitted key falls back to the consumer default")
	assert.Equal(t, 0, ReviewGateConf{MaxRevisions: new(0)}.ResolveMaxRevisions(3),
		"an explicit zero means no additional revisions")
	assert.Equal(t, 0, ReviewGateConf{MaxRevisions: new(-2)}.ResolveMaxRevisions(3),
		"a negative budget clamps to a single round instead of the default")
	assert.Equal(t, 7, ReviewGateConf{MaxRevisions: new(7)}.ResolveMaxRevisions(3))
}

func writeReviewGateConfig(t *testing.T, reviewGate string) *HarnessConfig {
	t.Helper()
	dir := t.TempDir()
	content := "mode: full\nproject_name: test\nplatforms:\n  - claude-code\nspec:\n" +
		"  id_format: \"SPEC-{DOMAIN}-{NUMBER}\"\n  review_gate:\n    enabled: true\n" + reviewGate
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0o644))

	cfg, err := Load(dir)
	require.NoError(t, err)
	return cfg
}

func TestLoad_ReviewGateMaxRevisionsExplicitZeroSurvivesDecode(t *testing.T) {
	t.Parallel()

	cfg := writeReviewGateConfig(t, "    max_revisions: 0\n")

	require.NotNil(t, cfg.Spec.ReviewGate.MaxRevisions,
		"an explicitly configured zero must stay distinguishable from an omitted key")
	assert.Equal(t, 0, *cfg.Spec.ReviewGate.MaxRevisions)
	assert.Equal(t, 0, cfg.Spec.ReviewGate.ResolveMaxRevisions(3))
}

func TestLoad_ReviewGateMaxRevisionsOmittedKeyKeepsFallback(t *testing.T) {
	t.Parallel()

	cfg := writeReviewGateConfig(t, "    judge: claude\n")

	assert.Nil(t, cfg.Spec.ReviewGate.MaxRevisions)
	assert.Equal(t, 3, cfg.Spec.ReviewGate.ResolveMaxRevisions(3))
}

func TestSave_ReviewGateMaxRevisionsRoundTrips(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := DefaultFullConfig("round-trip")
	cfg.Spec.ReviewGate.MaxRevisions = new(0)
	require.NoError(t, Save(dir, cfg))

	reloaded, err := Load(dir)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Spec.ReviewGate.MaxRevisions)
	assert.Equal(t, 0, *reloaded.Spec.ReviewGate.MaxRevisions,
		"a saved explicit zero must not come back as an omitted key")
}
