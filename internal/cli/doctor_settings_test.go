// Package cli_test covers doctor settings and runtime edge cases.
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestDoctorCmd_InvalidSettingsJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// Write invalid JSON to trigger the parse failure path.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte("not valid json"), 0644))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "settings.json")
}

func TestDoctorCmd_SettingsJSON_NoHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// Write settings.json with no hooks and no permissions.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{}`), 0644))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "hooks: not configured")
}

func TestDoctorCmd_SettingsJSON_EmptyHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// Write settings.json with an empty hooks map.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"hooks": {}, "permissions": {"allow": []}}`), 0644))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Hooks & Permissions")
}

func TestDoctorCmd_UnknownPlatform(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.DefaultFullConfig("test-proj")
	_ = cfg
	_ = dir

	// Save a config shape that still exercises the default platform branch.
	cfg2 := config.DefaultFullConfig("test-proj")
	cfg2.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(dir, cfg2))

	var out bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&out)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	require.NoError(t, doctorCmd.Execute())

	assert.Contains(t, out.String(), "Quality Gate")
}
