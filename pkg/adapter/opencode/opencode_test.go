package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestNew_DefaultRoot(t *testing.T) {
	t.Parallel()
	a := New()
	assert.Equal(t, ".", a.root)
}

func TestNewWithRoot(t *testing.T) {
	t.Parallel()
	a := NewWithRoot("/some/path")
	assert.Equal(t, "/some/path", a.root)
}

func TestAdapter_Accessors(t *testing.T) {
	t.Parallel()
	a := New()
	assert.Equal(t, "opencode", a.Name())
	assert.Equal(t, "1.0.0", a.Version())
	assert.Equal(t, "opencode", a.CLIBinary())
	assert.True(t, a.SupportsHooks())
}

func TestAdapter_Detect_NoError(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	_, err := a.Detect(context.Background())
	assert.NoError(t, err)
}

func TestAdapter_Generate_CreatesOpenCodeFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")

	pf, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, pf.Files)

	assert.FileExists(t, filepath.Join(dir, "AGENTS.md"))
	assert.FileExists(t, filepath.Join(dir, "opencode.json"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "commands", "auto.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "commands", "auto-plan.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "planner.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "plugins", "autopus-hooks.js"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "planning", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "opencode-manifest.json"))

	agentsData, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentsData), markerBegin)
	assert.Contains(t, string(agentsData), "플랫폼")

	configDoc := readConfigJSON(t, filepath.Join(dir, "opencode.json"))
	instructions := jsonStringSlice(configDoc["instructions"])
	assert.NotEmpty(t, instructions)
	assert.Contains(t, instructions, ".opencode/rules/autopus/branding.md")
}

func TestAdapter_Generate_NilConfig(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	_, err := a.Generate(context.Background(), nil)
	assert.Error(t, err)
}

func TestAdapter_Update_PreservesMergedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Custom Header\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"share":"manual"}`), 0644))

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	updated, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, updated.Files)

	agentsData, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentsData), "# Custom Header")
	assert.Contains(t, string(agentsData), markerBegin)

	configDoc := readConfigJSON(t, filepath.Join(dir, "opencode.json"))
	assert.Equal(t, "manual", configDoc["share"])
	assert.Contains(t, jsonStringSlice(configDoc["instructions"]), ".opencode/rules/autopus/branding.md")
}

func TestAdapter_Validate_AfterGenerate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestAdapter_Clean_RemovesGeneratedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Custom Header\n"), 0644))
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	err = a.Clean(context.Background())
	require.NoError(t, err)

	assert.NoDirExists(t, filepath.Join(dir, ".opencode"))
	assert.NoFileExists(t, filepath.Join(dir, "opencode.json"))
	agentsData, readErr := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(agentsData), "# Custom Header")
	assert.NotContains(t, string(agentsData), markerBegin)
}

func TestAdapter_InstallHooks_WritesPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	hooks := []adapter.HookConfig{{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet --warn-only", Timeout: 30}}

	err := a.InstallHooks(context.Background(), hooks, nil)
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(dir, ".opencode", "plugins", "autopus-hooks.js"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "auto check --arch --quiet --warn-only")
}

func TestInjectOrchestraPlugin_MergesPluginArray(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"plugin":["existing-plugin"]}`), 0644))

	err := a.InjectOrchestraPlugin("/path/to/script.js")
	require.NoError(t, err)

	configDoc := readConfigJSON(t, filepath.Join(dir, "opencode.json"))
	plugins := jsonStringSlice(configDoc["plugin"])
	assert.Contains(t, plugins, "existing-plugin")
	assert.Contains(t, plugins, "/path/to/script.js")
	assert.Contains(t, jsonStringSlice(configDoc["instructions"]), ".opencode/rules/autopus/branding.md")
}

func TestInjectOrchestraPlugin_InvalidExistingJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), []byte("{broken"), 0644))

	err := a.InjectOrchestraPlugin("/path/to/script.js")
	assert.Error(t, err)
}

func readConfigJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	return doc
}
