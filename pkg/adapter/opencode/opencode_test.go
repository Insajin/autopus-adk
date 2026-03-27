package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAdapter_Detect_BinaryNotFound(t *testing.T) {
	t.Parallel()
	// Use a root that won't affect detection (LookPath checks PATH).
	a := NewWithRoot(t.TempDir())
	found, err := a.Detect(context.Background())
	assert.NoError(t, err)
	// We cannot guarantee opencode is NOT installed, but we verify no error.
	_ = found
}

func TestAdapter_Generate_ReturnsEmptyFiles(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	pf, err := a.Generate(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, pf)
	assert.Empty(t, pf.Files)
}

func TestAdapter_Update_DelegatesToGenerate(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	pf, err := a.Update(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, pf)
}

func TestAdapter_Validate_ReturnsNil(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	errs, err := a.Validate(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, errs)
}

func TestAdapter_Clean_RemovesConfigFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Create opencode.json
	cfgPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0644))

	err := a.Clean(context.Background())
	assert.NoError(t, err)

	_, statErr := os.Stat(cfgPath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestAdapter_Clean_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// No file to remove — should not error.
	err := a.Clean(context.Background())
	assert.NoError(t, err)
}

func TestAdapter_InstallHooks_Noop(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	err := a.InstallHooks(context.Background(), nil, nil)
	assert.NoError(t, err)
}

func TestInjectOrchestraPlugin_NewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	err := a.InjectOrchestraPlugin("/path/to/script.ts")
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(dir, "opencode.json"))
	require.NoError(t, readErr)

	var cfg opencodeConfig
	require.NoError(t, json.Unmarshal(data, &cfg))
	require.NotNil(t, cfg.Experimental)
	require.Len(t, cfg.Experimental.Plugins, 1)
	assert.Equal(t, "autopus-result", cfg.Experimental.Plugins[0].Name)
	assert.Equal(t, "text.complete", cfg.Experimental.Plugins[0].Event)
	assert.Equal(t, "bun /path/to/script.ts", cfg.Experimental.Plugins[0].Command)
}

func TestInjectOrchestraPlugin_PreservesExistingPlugins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Write existing config with a user plugin.
	existing := opencodeConfig{
		Experimental: &experimentalBlock{
			Plugins: []pluginEntry{
				{Name: "user-plugin", Event: "text.complete", Command: "echo hello"},
			},
		},
	}
	data, _ := json.Marshal(existing)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0644))

	err := a.InjectOrchestraPlugin("/path/to/script.ts")
	require.NoError(t, err)

	updated, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	var cfg opencodeConfig
	require.NoError(t, json.Unmarshal(updated, &cfg))

	// Both user plugin and autopus plugin must exist.
	require.Len(t, cfg.Experimental.Plugins, 2)
	assert.Equal(t, "user-plugin", cfg.Experimental.Plugins[0].Name)
	assert.Equal(t, "autopus-result", cfg.Experimental.Plugins[1].Name)
}

func TestInjectOrchestraPlugin_DeduplicatesAutopusPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Inject twice — should not create duplicate.
	require.NoError(t, a.InjectOrchestraPlugin("/v1/script.ts"))
	require.NoError(t, a.InjectOrchestraPlugin("/v2/script.ts"))

	data, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	var cfg opencodeConfig
	require.NoError(t, json.Unmarshal(data, &cfg))

	// Only one autopus-result entry with updated command.
	require.Len(t, cfg.Experimental.Plugins, 1)
	assert.Equal(t, "bun /v2/script.ts", cfg.Experimental.Plugins[0].Command)
}

func TestInjectOrchestraPlugin_InvalidExistingJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Write invalid JSON.
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "opencode.json"),
		[]byte("{broken"),
		0644,
	))

	err := a.InjectOrchestraPlugin("/path/to/script.ts")
	assert.Error(t, err, "must return error for invalid JSON")
}
