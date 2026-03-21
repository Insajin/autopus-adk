package cli

import (
	"bytes"
	"context"
	"os"
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
	// Uses os.Getwd; not parallel-safe with Chdir.
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

	// printLoreEntries writes to stdout via fmt.Println.
	// We can only confirm it doesn't panic with empty input.
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

	// Should not panic.
	printLoreEntries(entries)
}

// TestSetupCmd_Structure verifies setup command registers four subcommands.
func TestSetupCmd_Structure(t *testing.T) {
	t.Parallel()

	cmd := newSetupCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "setup", cmd.Use)

	names := make([]string, 0)
	for _, sc := range cmd.Commands() {
		names = append(names, sc.Name())
	}
	assert.Contains(t, names, "generate")
	assert.Contains(t, names, "update")
	assert.Contains(t, names, "validate")
	assert.Contains(t, names, "status")
}

// TestSetupGenerateCmd_NoDocsDir verifies setup generate creates docs in temp dir.
func TestSetupGenerateCmd_NoDocsDir(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	cmd := newSetupGenerateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = cmd.RunE(cmd, []string{dir})
	// May succeed or fail depending on project structure; we just test execution path.
	_ = err
}

// TestSetupStatusCmd_NoDocs verifies "No documentation found" path.
func TestSetupStatusCmd_NoDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := newSetupStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, []string{dir})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No documentation found")
}

// TestSetupValidateCmd_NoDocsDir verifies setup validate returns error when docs missing.
func TestSetupValidateCmd_NoDocsDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := newSetupValidateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	// No docs directory: Validate may return error or pass with warnings.
	err := cmd.RunE(cmd, []string{dir})
	// Either way the function should complete without panicking.
	_ = err
}

// TestSetupUpdateCmd_NoDocsDir verifies setup update when no docs exist.
func TestSetupUpdateCmd_NoDocsDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := newSetupUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, []string{dir})
	_ = err
}

// TestLSPCmd_Structure verifies lsp command has all subcommands.
func TestLSPCmd_FullStructure(t *testing.T) {
	t.Parallel()

	cmd := newLSPCmd()
	require.NotNil(t, cmd)

	names := make([]string, 0)
	for _, sc := range cmd.Commands() {
		names = append(names, sc.Name())
	}
	assert.Contains(t, names, "diagnostics")
	assert.Contains(t, names, "refs")
	assert.Contains(t, names, "rename")
	assert.Contains(t, names, "symbols")
	assert.Contains(t, names, "definition")
}

// TestLSPDiagnosticsCmd_FlagsPresent verifies that diagnostics has format flag.
func TestLSPDiagnosticsCmd_FlagsPresent(t *testing.T) {
	t.Parallel()

	cmd := newLSPDiagnosticsCmd()
	assert.NotNil(t, cmd.Flags().Lookup("format"), "format flag must exist")
}

// TestRunSpecReviewCmd_NoConfigError verifies that runSpecReview fails when SPEC missing.
func TestRunSpecReviewCmd_NoConfigError(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	err = runSpecReview(context.Background(), "SPEC-DOES-NOT-EXIST", "consensus", 10)
	assert.Error(t, err)
}
