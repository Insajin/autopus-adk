package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/lore"
)

// TestResolveDirFromArgs_WithArg verifies that resolveDirFromArgs returns the dir from args.
func TestResolveDirFromArgs_WithArg(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result, err := resolveDirFromArgs([]string{dir})
	require.NoError(t, err)
	assert.Equal(t, dir, result)
}

// TestResolveDirFromArgs_NoArgs uses current directory fallback.
func TestResolveDirFromArgs_NoArgs(t *testing.T) {
	result, err := resolveDirFromArgs([]string{})
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

// TestResolveOutputDir_WithExplicit verifies that explicit outputDir is returned as-is.
func TestResolveOutputDir_WithExplicit(t *testing.T) {
	t.Parallel()

	result := resolveOutputDir("/some/project", "/custom/output")
	assert.Equal(t, "/custom/output", result)
}

// TestResolveOutputDir_DefaultFallback verifies the default .autopus/docs path.
func TestResolveOutputDir_DefaultFallback(t *testing.T) {
	t.Parallel()

	result := resolveOutputDir("/some/project", "")
	assert.Equal(t, "/some/project/.autopus/docs", result)
}

// TestPrintLoreEntries_Empty verifies that empty entries prints "항목 없음".
func TestPrintLoreEntries_Empty(t *testing.T) {
	t.Parallel()

	printLoreEntries(nil)
	printLoreEntries([]lore.LoreEntry{})
}

// TestPrintLoreEntries_WithEntries verifies that entries are printed without panic.
func TestPrintLoreEntries_WithEntries(t *testing.T) {
	t.Parallel()

	entries := []lore.LoreEntry{
		{
			CommitMsg:     "feat: add test feature",
			Constraint:    "must not break API",
			Rejected:      "option A",
			Confidence:    "high",
			ScopeRisk:     "local",
			Reversibility: "trivial",
			Directive:     "follow TDD",
			Tested:        "unit tests",
			NotTested:     "e2e tests",
			Related:       "SPEC-001",
		},
		{
			CommitMsg: "fix: simple fix",
		},
	}

	printLoreEntries(entries)
}
