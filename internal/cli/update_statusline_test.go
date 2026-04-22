package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCmd_StatusLineModeMergeCombinesExistingCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code", "--yes"})
	require.NoError(t, initCmd.Execute())

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{
  "statusLine": {
    "type": "command",
    "command": "node ~/.claude/hud/omc-hud.mjs",
    "padding": 2
  }
}`), 0o644))

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetErr(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--yes", "--statusline-mode", "merge"})
	require.NoError(t, updateCmd.Execute())

	updated, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Contains(t, string(updated), ".claude/statusline-combined.sh")

	userCommand, err := os.ReadFile(filepath.Join(dir, ".claude", "statusline-user-command.txt"))
	require.NoError(t, err)
	assert.Equal(t, "node ~/.claude/hud/omc-hud.mjs\n", string(userCommand))

	assert.FileExists(t, filepath.Join(dir, ".claude", "statusline-combined.sh"))
	assert.Contains(t, out.String(), "merged existing command + Autopus")
}
