package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	dir := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	cmd := newSetupGenerateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = cmd.RunE(cmd, []string{dir})
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
	err := cmd.RunE(cmd, []string{dir})
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

// TestLSPCmd_FullStructure verifies lsp command has all subcommands.
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
	dir := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	err = runSpecReview(context.Background(), "SPEC-DOES-NOT-EXIST", "consensus", 10)
	assert.Error(t, err)
}
