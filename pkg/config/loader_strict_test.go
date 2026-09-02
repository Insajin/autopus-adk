package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const strictBaseConfig = "project_name: strict\nmode: full\nplatforms:\n  - claude-code\n"

func writeStrictConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, configFileName), []byte(body), 0o644))
	return root
}

// A misspelled key used to load successfully while having no effect, and Save
// then dropped it from disk. Loading must fail loudly enough that the user can
// see which key and which line are wrong.
func TestLoadPreview_RejectsUnknownKeyWithLocatableError(t *testing.T) {
	t.Parallel()
	root := writeStrictConfig(t, strictBaseConfig+"language:\n  comments: en\n  ai_response: ko\n")

	_, err := LoadPreview(root)

	require.Error(t, err)
	assert.ErrorContains(t, err, "parse config")
	assert.ErrorContains(t, err, "ai_response")
	assert.ErrorContains(t, err, "line 7")
	assert.ErrorContains(t, err, "unknown keys are rejected")
}

// The typo must not be papered over by a neighbouring valid key in the same
// block: the block still decodes, so only strict decoding catches it.
func TestLoadPreview_UnknownKeyRejectionSurvivesValidSiblings(t *testing.T) {
	t.Parallel()
	root := writeStrictConfig(t, strictBaseConfig+"spec:\n  id_format: SPEC-{DOMAIN}-{NUMBER}\n  ears_type: [ubiquitous]\n")

	_, err := LoadPreview(root)

	require.ErrorContains(t, err, "ears_type")
}

// Unknown top-level keys are rejected on the same terms as nested ones.
func TestLoadPreview_RejectsUnknownTopLevelKey(t *testing.T) {
	t.Parallel()
	root := writeStrictConfig(t, strictBaseConfig+"platfroms:\n  - codex\n")

	_, err := LoadPreview(root)

	require.ErrorContains(t, err, "platfroms")
}

// Removed keys are the only opt-out and they are named in removedConfigKeys.
// A workspace still carrying one must keep loading, and its siblings must
// still apply.
func TestLoadPreview_AcceptsExplicitlyRemovedKeys(t *testing.T) {
	t.Parallel()
	for _, key := range removedConfigKeys {
		assert.NotEmpty(t, key, "removed key entries must be dotted paths")
	}
	root := writeStrictConfig(t, strictBaseConfig+"workflow:\n  team_default: false\n  coverage_threshold: 91\n")

	cfg, err := LoadPreview(root)

	require.NoError(t, err)
	assert.Equal(t, 91, cfg.Workflow.CoverageThreshold)
}

// Pruning a removed key must not consume the sibling that follows it in the
// same mapping, which is the failure mode of index-shifting deletions.
func TestLoadPreview_RemovedKeyPruningKeepsFollowingSibling(t *testing.T) {
	t.Parallel()
	root := writeStrictConfig(t, strictBaseConfig+"workflow:\n  coverage_threshold: 77\n  team_default: true\n")

	cfg, err := LoadPreview(root)

	require.NoError(t, err)
	assert.Equal(t, 77, cfg.Workflow.CoverageThreshold)
}
