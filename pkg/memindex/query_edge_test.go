package memindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusCorruptProjection asserts Status reports a corrupt state (instead
// of erroring) when the index file exists but is not a valid projection.
func TestStatusCorruptProjection(t *testing.T) {
	t.Parallel()
	projectDir := t.TempDir()

	// Write a non-empty, non-sqlite file at the resolved index path so
	// openExisting passes the size check but verifyProjection/queries fail.
	runtimeRoot := filepath.Join(projectDir, ".autopus", "runtime", "memindex")
	require.NoError(t, os.MkdirAll(runtimeRoot, 0o755))
	corrupt := filepath.Join(runtimeRoot, "corrupt.sqlite")
	require.NoError(t, os.WriteFile(corrupt, []byte("not a sqlite database file"), 0o644))

	status, err := Status(Options{ProjectDir: projectDir, IndexPath: "corrupt.sqlite"})
	require.NoError(t, err)
	assert.True(t, status.CorruptState.IsCorrupt)
	assert.NotEmpty(t, status.CorruptState.Reason)
	assert.True(t, status.RebuildRecommended)
}

// TestContextDefaultBudgetAndTopK covers the Context default-value branches
// (TopK<=0 and BudgetTokens<=0) against a real rebuilt index, asserting the
// prompt and budget reflect the applied defaults.
func TestContextDefaultBudgetAndTopK(t *testing.T) {
	t.Parallel()
	projectDir := makeMemIndexFixture(t)
	_, err := Rebuild(Options{ProjectDir: projectDir, IndexPath: "ctx.sqlite"})
	require.NoError(t, err)

	context, err := Context(ContextOptions{
		ProjectDir: projectDir,
		IndexPath:  "ctx.sqlite",
		Query:      "approval drift stable role selectors",
		// TopK and BudgetTokens omitted -> defaults 20 and 800.
	})
	require.NoError(t, err)
	assert.Equal(t, 800, context.BudgetTokens)
	assert.Contains(t, context.Prompt, "## Quality Recall")
	assert.NotEmpty(t, context.Results)
}

// TestSearchNoMatchEmptyResults asserts a well-formed but non-matching query
// returns an empty result set rather than an error.
func TestSearchNoMatchEmptyResults(t *testing.T) {
	t.Parallel()
	projectDir := makeMemIndexFixture(t)
	_, err := Rebuild(Options{ProjectDir: projectDir, IndexPath: "nomatch.sqlite"})
	require.NoError(t, err)

	search, err := Search(SearchOptions{
		ProjectDir: projectDir,
		IndexPath:  "nomatch.sqlite",
		Query:      "zzqqxx nonexistent token zzqqxx",
		TopK:       5,
	})
	require.NoError(t, err)
	assert.Empty(t, search.Results)
	assert.Equal(t, SchemaVersion, search.SchemaVersion)
}
