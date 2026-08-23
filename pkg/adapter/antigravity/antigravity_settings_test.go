package antigravity_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/antigravity"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestAntigravityGenerateSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := antigravity.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Verify settings.json exists and is valid JSON
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	err = json.Unmarshal(data, &settings)
	assert.NoError(t, err, "settings.json should be valid JSON")
}

func TestAntigravitySettingsMCPServers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := antigravity.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Check mcpServers contains context7
	mcpServers, ok := settings["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers should be a map")
	_, hasContext7 := mcpServers["context7"]
	assert.True(t, hasContext7, "mcpServers should contain context7")
}

func TestAntigravitySupportsHooksTrue(t *testing.T) {
	t.Parallel()
	a := antigravity.New()
	assert.True(t, a.SupportsHooks(), "SupportsHooks should return true")
}

func TestAntigravitySettingsMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create existing settings.json with user-defined key
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))

	existingSettings := map[string]any{
		"userKey": "userValue",
		"mcpServers": map[string]any{
			"custom-server": map[string]any{
				"command": "my-server",
			},
		},
	}
	existingJSON, _ := json.MarshalIndent(existingSettings, "", "  ")
	require.NoError(t, os.WriteFile(
		filepath.Join(settingsDir, "settings.json"),
		existingJSON, 0644,
	))

	a := antigravity.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Read merged settings
	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	require.NoError(t, err)

	var merged map[string]any
	require.NoError(t, json.Unmarshal(data, &merged))

	// User key should be preserved
	assert.Equal(t, "userValue", merged["userKey"],
		"user-defined keys should be preserved after merge")

	// Both custom-server and context7 should exist in mcpServers
	mcpServers, ok := merged["mcpServers"].(map[string]any)
	require.True(t, ok)
	_, hasCustom := mcpServers["custom-server"]
	_, hasContext7 := mcpServers["context7"]
	assert.True(t, hasCustom, "custom-server should be preserved")
	assert.True(t, hasContext7, "context7 should be added")
}

func TestAntigravityInstallHooksCreatesSettingsJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := antigravity.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Verify settings.json was created
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	_, statErr := os.Stat(settingsPath)
	assert.NoError(t, statErr, ".gemini/settings.json should exist")
}
