package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectOrchestraAfterAgentHook_Dedup_SamePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Inject twice with the same path
	require.NoError(t, a.InjectOrchestraAfterAgentHook("/path/to/collect.sh"))
	require.NoError(t, a.InjectOrchestraAfterAgentHook("/path/to/collect.sh"))

	data, err := os.ReadFile(filepath.Join(dir, ".gemini", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	hooksMap := settings["hooks"].(map[string]any)
	afterAgent := hooksMap["AfterAgent"].([]any)
	assert.Len(t, afterAgent, 1, "duplicate command should not be added")
}

func TestInjectOrchestraAfterAgentHook_Dedup_DifferentPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	require.NoError(t, a.InjectOrchestraAfterAgentHook("/path/to/collect-a.sh"))
	require.NoError(t, a.InjectOrchestraAfterAgentHook("/path/to/collect-b.sh"))

	data, err := os.ReadFile(filepath.Join(dir, ".gemini", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	hooksMap := settings["hooks"].(map[string]any)
	afterAgent := hooksMap["AfterAgent"].([]any)
	assert.Len(t, afterAgent, 2, "different commands should both be added")
}

func TestInjectOrchestraAfterAgentHook_Dedup_PreservesOtherHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Pre-populate with existing user hook and autopus hook
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))

	existing := map[string]any{
		"hooks": map[string]any{
			"AfterAgent": []any{
				map[string]any{"command": "user-hook.sh"},
				map[string]any{"command": "/path/to/collect.sh"},
			},
		},
	}
	data, _ := json.Marshal(existing)
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644))

	// Try to inject same path — should be skipped
	require.NoError(t, a.InjectOrchestraAfterAgentHook("/path/to/collect.sh"))

	updated, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(updated, &settings))

	hooksMap := settings["hooks"].(map[string]any)
	afterAgent := hooksMap["AfterAgent"].([]any)
	assert.Len(t, afterAgent, 2, "no duplicate added, user hook preserved")
}
