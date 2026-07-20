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

// TestIsHookModeAvailable_NestedWorkingDirectoryFindsAncestorProjectHook
// verifies that orchestra hook detection follows the owning project root when
// the command is launched from a nested module or background terminal cwd.
func TestIsHookModeAvailable_NestedWorkingDirectoryFindsAncestorProjectHook(t *testing.T) {
	// Given: only the ancestor project has an Autopus Stop hook. Keep HOME
	// isolated so a real user-global Claude configuration cannot satisfy the test.
	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(projectRoot, "autopus.yaml"),
		[]byte("project: nested-hook-test\n"),
		0o600,
	))
	claudeDir := filepath.Join(projectRoot, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"autopus hook stop"}]}]}}`),
		0o600,
	))

	nestedDir := filepath.Join(projectRoot, "modules", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o700))
	t.Setenv("HOME", t.TempDir())
	t.Chdir(nestedDir)

	// When: hook availability is resolved from the nested working directory.
	available := isHookModeAvailable()

	// Then: the ancestor project's hook keeps structured orchestra pane-capable.
	assert.True(t, available, "nested orchestra cwd must discover the ancestor project hook")
}

// TestIsHookModeAvailable_CodexOnlyProject verifies that a Codex-native Stop
// hook is sufficient to keep structured orchestra on the pane path. Codex-only
// installs must not depend on a sibling Claude settings file.
func TestIsHookModeAvailable_CodexOnlyProject(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(projectRoot, "autopus.yaml"),
		[]byte("project: codex-only-hook-test\n"),
		0o600,
	))
	codexDir := filepath.Join(projectRoot, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(codexDir, "hooks.json"),
		[]byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":".codex/hooks/autopus/hook-codex-stop.sh"}]}]}}`),
		0o600,
	))

	nestedDir := filepath.Join(projectRoot, "modules", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o700))
	t.Setenv("HOME", t.TempDir())
	t.Chdir(nestedDir)

	assert.True(t, isHookModeAvailable(),
		"Codex Stop hooks must enable pane completion without .claude/settings.json")
}

// TestIsHookModeAvailable_GlobalCodexHook verifies user-global Codex hooks are
// detected in the same way as user-global Claude hooks.
func TestIsHookModeAvailable_GlobalCodexHook(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(codexDir, "hooks.json"),
		[]byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":".codex/hooks/autopus/hook-codex-stop.sh"}]}]}}`),
		0o600,
	))
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	assert.True(t, isHookModeAvailable(), "global Codex Stop hook must enable pane completion")
}

// TestIsHookModeAvailable_StopsAtNearestAutopusProjectRoot verifies that a
// hook belonging to an unrelated ancestor workspace is not treated as the
// current project's hook.
func TestIsHookModeAvailable_StopsAtNearestAutopusProjectRoot(t *testing.T) {
	// Given: the nearest Autopus project has no hook, while its parent does.
	ancestor := t.TempDir()
	ancestorClaudeDir := filepath.Join(ancestor, ".claude")
	require.NoError(t, os.MkdirAll(ancestorClaudeDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(ancestorClaudeDir, "settings.json"),
		[]byte(`{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"autopus hook stop"}]}]}}`),
		0o600,
	))

	projectRoot := filepath.Join(ancestor, "current-project")
	require.NoError(t, os.MkdirAll(projectRoot, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectRoot, "autopus.yaml"),
		[]byte("project: current-project\n"),
		0o600,
	))
	nestedDir := filepath.Join(projectRoot, "modules", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o700))
	t.Setenv("HOME", t.TempDir())
	t.Chdir(nestedDir)

	// When: hook availability is resolved inside the marker-bounded project.
	available := isHookModeAvailable()

	// Then: the unrelated ancestor hook must not enable hook mode.
	assert.False(t, available, "hook discovery must stop at the nearest autopus.yaml project root")
}
