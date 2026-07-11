package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestUpdate_UserMarkerPreservesOnlyNamedCodexSetting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	useFullCodexCatalogForTest(a)
	cfg := config.DefaultFullConfig("test-project")
	cfg.Quality.SupervisorModelPolicy = "quality"

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	configPath := filepath.Join(dir, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	custom := strings.Replace(string(data),
		`model_reasoning_summary = "auto"`,
		`model_reasoning_summary = "detailed"`,
		1,
	)
	require.NotEqual(t, string(data), custom)
	require.NoError(t, os.WriteFile(configPath, []byte(custom), 0o644))

	_, err = a.Update(context.Background(), cfg)
	require.NoError(t, err)
	marked, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(marked), codexUserModelMarker+": model_reasoning_summary")

	cfg.Quality.Default = "ultra"
	_, err = a.Update(context.Background(), cfg)
	require.NoError(t, err)
	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	root := strings.SplitN(string(updated), "[agents]", 2)[0]
	assert.Contains(t, root, `model = "gpt-5.6-sol"`)
	assert.Contains(t, root, `model_reasoning_effort = "ultra"`)
	assert.Contains(t, root, `model_reasoning_summary = "detailed"`)
	assert.NotContains(t, root, codexUserModelMarker+": model,")
}
