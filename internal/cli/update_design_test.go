package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCmd_BackfillsDesignContextForExistingHarness(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(`mode: full
project_name: legacy-proj
platforms:
  - claude-code
verify:
  enabled: true
`), 0o644))

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--yes"})
	require.NoError(t, updateCmd.Execute())

	configData, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configData), "design:")
	assert.Contains(t, string(configData), "inject_on_verify: true")

	designData, err := os.ReadFile(filepath.Join(dir, "DESIGN.md"))
	require.NoError(t, err)
	assert.Contains(t, string(designData), "# DESIGN.md")
}

func TestUpdateCmd_DoesNotCreateDesignWhenDisabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(`mode: full
project_name: no-design-proj
platforms:
  - claude-code
design:
  enabled: false
`), 0o644))

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--yes"})
	require.NoError(t, updateCmd.Execute())

	assert.NoFileExists(t, filepath.Join(dir, "DESIGN.md"))
}
