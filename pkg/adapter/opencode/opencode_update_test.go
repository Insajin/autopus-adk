package opencode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestAdapter_Update_NoManifestWriteFailureRollsBackCreatedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")

	blockerPath := filepath.Join(dir, ".opencode", "commands", "auto.md")
	require.NoError(t, os.MkdirAll(blockerPath, 0755))

	_, err := a.Update(context.Background(), cfg)

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "AGENTS.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".opencode", "commands", "auto.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "opencode-manifest.json"))
	assert.DirExists(t, blockerPath)
}

func TestAdapter_Update_WithManifestWriteFailureRollsBackWritesAndPrunes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := NewWithRoot(dir)
	fullCfg := config.DefaultFullConfig("demo")
	fullCfg.Platforms = []string{"opencode"}

	_, err := a.Generate(context.Background(), fullCfg)
	require.NoError(t, err)

	agentsPath := filepath.Join(dir, "AGENTS.md")
	beforeAgents, err := os.ReadFile(agentsPath)
	require.NoError(t, err)

	blockerPath := filepath.Join(dir, ".opencode", "commands", "auto.md")
	require.NoError(t, os.Remove(blockerPath))
	require.NoError(t, os.MkdirAll(blockerPath, 0755))

	mixedCfg := config.DefaultFullConfig("demo")
	mixedCfg.Platforms = []string{"codex", "opencode"}
	mixedCfg.Skills.SharedSurface = config.SharedSurfaceAuto
	_, err = a.Update(context.Background(), mixedCfg)

	require.Error(t, err)
	afterAgents, readErr := os.ReadFile(agentsPath)
	require.NoError(t, readErr)
	assert.Equal(t, string(beforeAgents), string(afterAgents))
	assert.DirExists(t, blockerPath)
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "metrics", "SKILL.md"))
}
