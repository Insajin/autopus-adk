package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareAntigravityHooksJSON_UsesOfficialSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	hooks := []adapter.HookConfig{{
		Event:   "PreToolUse",
		Matcher: "run_command",
		Type:    "command",
		Command: "auto check --arch --quiet --warn-only",
		Timeout: 30,
	}}

	files, err := a.prepareAntigravityHooksJSON(hooks)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, ".agents/hooks.json", files[0].TargetPath)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(files[0].Content, &parsed))
	autopus := parsed["autopus"].(map[string]any)
	entries := autopus["PreToolUse"].([]any)
	first := entries[0].(map[string]any)
	assert.Equal(t, "run_command", first["matcher"])
}

func TestPrepareAntigravityPluginJSON(t *testing.T) {
	t.Parallel()

	files, err := prepareAntigravityPluginJSON()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, ".agents/plugins/autopus/plugin.json", files[0].TargetPath)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(files[0].Content, &parsed))
	assert.Equal(t, "autopus", parsed["name"])
}

func TestMirrorAntigravityPluginMappings(t *testing.T) {
	t.Parallel()

	files := mirrorAntigravityPluginMappings([]adapter.FileMapping{{
		TargetPath: ".gemini/commands/auto/plan.toml",
		Content:    []byte("Load .gemini/skills/autopus/auto-plan/SKILL.md"),
	}})

	require.Len(t, files, 1)
	assert.Equal(t, ".agents/plugins/autopus/commands/auto/plan.toml", files[0].TargetPath)
	assert.Contains(t, string(files[0].Content), ".agents/plugins/autopus/skills/auto-plan/SKILL.md")
}

func TestGenerate_CreatesAntigravityHooksJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".agents", "hooks.json"))
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Contains(t, parsed, "autopus")
}

func TestGenerate_CreatesAntigravityPluginSurface(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".agents", "plugins", "autopus", "plugin.json"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "plugins", "autopus", "skills", "auto-plan", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "plugins", "autopus", "rules", "lore-commit.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "plugins", "autopus", "agents", "executor.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "plugins", "autopus", "commands", "auto", "plan.toml"))

	commandData, err := os.ReadFile(filepath.Join(dir, ".agents", "plugins", "autopus", "commands", "auto", "plan.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(commandData), ".agents/plugins/autopus/skills/auto-plan/SKILL.md")
}

func TestRemoveAntigravityHooksJSON_PreservesUserHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	hooksDir := filepath.Join(dir, ".agents")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	data, _ := json.Marshal(map[string]any{
		"autopus": map[string]any{"enabled": true},
		"user":    map[string]any{"enabled": true},
	})
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hooks.json"), data, 0644))

	require.NoError(t, a.removeAntigravityHooksJSON())

	updated, err := os.ReadFile(filepath.Join(hooksDir, "hooks.json"))
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	assert.NotContains(t, parsed, "autopus")
	assert.Contains(t, parsed, "user")
}
