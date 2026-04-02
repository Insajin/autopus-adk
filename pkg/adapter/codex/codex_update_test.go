package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate_NoManifest_FallsBackToGenerate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
	_, statErr := os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.NoError(t, statErr)
}

func TestUpdate_WithManifest_WritesNewFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	cfg.ProjectName = "updated-project"
	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
	assert.NotEmpty(t, pf.Files)

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated-project")
}

func TestUpdate_UserModifiedFile_BackedUp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Modify managed files to trigger backup
	skillsDir := filepath.Join(dir, ".codex", "skills")
	entries, _ := os.ReadDir(skillsDir)
	if len(entries) > 0 {
		targetFile := filepath.Join(skillsDir, entries[0].Name())
		require.NoError(t, os.WriteFile(targetFile, []byte("user modified content"), 0644))
	}
	configPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("user changed config"), 0644))

	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
	assert.NotEmpty(t, pf.Files)
}

func TestUpdate_DeletedManagedFile_Skipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	skillsDir := filepath.Join(dir, ".codex", "skills")
	entries, _ := os.ReadDir(skillsDir)
	if len(entries) > 0 {
		require.NoError(t, os.Remove(filepath.Join(skillsDir, entries[0].Name())))
	}

	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
}
