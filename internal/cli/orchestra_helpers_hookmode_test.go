// Package cli tests hook-mode availability detection for project-local
// settings.json support (SPEC-ORCH-022 REQ-003, oracle S3).
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHookModeAvailableInDirs_ProjectLocalSettingsHits verifies that a
// project-local .claude/settings.json containing both "autopus" and "Stop"
// makes hookModeAvailableInDirs return true even when the global path is empty.
func TestHookModeAvailableInDirs_ProjectLocalSettingsHits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o700))

	settingsPath := filepath.Join(claudeDir, "settings.json")
	content := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"autopus hook stop"}]}]}}`
	require.NoError(t, os.WriteFile(settingsPath, []byte(content), 0o600))

	// Global path does not exist; project-local path has the required strings.
	ok := hookModeAvailableInDirs("/nonexistent/global/settings.json", settingsPath)
	assert.True(t, ok, "project-local settings with autopus+Stop must return true")
}

// TestHookModeAvailableInDirs_GlobalSettingsHits verifies the original user-global
// path still works when project-local path is absent.
func TestHookModeAvailableInDirs_GlobalSettingsHits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "settings.json")
	content := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"autopus hook stop"}]}]}}`
	require.NoError(t, os.WriteFile(globalPath, []byte(content), 0o600))

	ok := hookModeAvailableInDirs(globalPath, "/nonexistent/project/settings.json")
	assert.True(t, ok, "global settings with autopus+Stop must return true")
}

// TestHookModeAvailableInDirs_MissingStopKeyReturnsFalse verifies that having
// "autopus" without the "Stop" key results in false.
func TestHookModeAvailableInDirs_MissingStopKeyReturnsFalse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	// Contains "autopus" but no "Stop" key.
	content := `{"hooks":{"PreToolUse":[{"matcher":"","hooks":[{"type":"command","command":"autopus hook pre"}]}]}}`
	require.NoError(t, os.WriteFile(settingsPath, []byte(content), 0o600))

	ok := hookModeAvailableInDirs(settingsPath, "/nonexistent/project/settings.json")
	assert.False(t, ok, "settings missing Stop key must return false")
}

// TestHookModeAvailableInDirs_BothPathsMissingReturnsFalse verifies graceful
// degradation when neither settings file exists.
func TestHookModeAvailableInDirs_BothPathsMissingReturnsFalse(t *testing.T) {
	t.Parallel()

	ok := hookModeAvailableInDirs(
		"/nonexistent/global/settings.json",
		"/nonexistent/project/settings.json",
	)
	assert.False(t, ok, "both missing paths must return false without panicking")
}
