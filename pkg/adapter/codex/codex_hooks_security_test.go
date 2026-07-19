package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHooks_RejectsHooksJSONSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".codex"), 0o755))
	victim := filepath.Join(t.TempDir(), "victim.json")
	require.NoError(t, os.WriteFile(victim, []byte("preserve-hooks-json"), 0o600))
	requireSymlink(t, victim, filepath.Join(root, ".codex", "hooks.json"))

	_, err := NewWithRoot(root).generateHooks(config.DefaultFullConfig("test"))
	require.Error(t, err)
	assertFileContent(t, victim, "preserve-hooks-json")
}

func TestGenerateHooks_RejectsHookAssetSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	assetDir := filepath.Join(root, ".codex", "hooks", "autopus")
	require.NoError(t, os.MkdirAll(assetDir, 0o755))
	victim := filepath.Join(t.TempDir(), "victim.sh")
	require.NoError(t, os.WriteFile(victim, []byte("preserve-asset"), 0o600))
	requireSymlink(t, victim, filepath.Join(assetDir, "hook-codex-stop.sh"))

	_, err := NewWithRoot(root).generateHooks(config.DefaultFullConfig("test"))
	require.Error(t, err)
	assertFileContent(t, victim, "preserve-asset")
}

func TestGenerateHooks_RejectsSymlinkedHookParent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".codex"), 0o755))
	outside := t.TempDir()
	requireSymlink(t, outside, filepath.Join(root, ".codex", "hooks"))

	_, err := NewWithRoot(root).generateHooks(config.DefaultFullConfig("test"))
	require.Error(t, err)
	_, statErr := os.Stat(filepath.Join(outside, "autopus", "hook-codex-stop.sh"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestGenerateHooks_RepairsHookAssetMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	assetPath := filepath.Join(root, ".codex", "hooks", "autopus", "hook-codex-stop.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(assetPath), 0o755))
	require.NoError(t, os.WriteFile(assetPath, []byte("stale"), 0o644))
	require.NoError(t, os.Chmod(assetPath, 0o644))

	_, err := NewWithRoot(root).generateHooks(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	info, err := os.Stat(assetPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func requireSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
}
