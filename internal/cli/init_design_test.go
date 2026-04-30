package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_CreatesDesignContextStarter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code", "--yes"})
	require.NoError(t, cmd.Execute())

	designData, err := os.ReadFile(filepath.Join(dir, "DESIGN.md"))
	require.NoError(t, err)
	assert.Contains(t, string(designData), "# DESIGN.md")

	configData, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configData), "design:")
	assert.Contains(t, string(configData), "inject_on_review: true")
}

func TestInitCmd_PreservesExistingDesignContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "DESIGN.md"), []byte("# Product Design\n\nKeep this."), 0o644))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code", "--yes"})
	require.NoError(t, cmd.Execute())

	designData, err := os.ReadFile(filepath.Join(dir, "DESIGN.md"))
	require.NoError(t, err)
	assert.Contains(t, string(designData), "Keep this.")
}
