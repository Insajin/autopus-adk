package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_CreatesMissingWorkspaceRootAndRejectsNonDirectory(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "missing", "workspace")
	cfg := config.DefaultFullConfig("omp-missing-root")
	cfg.Platforms = []string{"omp"}

	_, err := NewWithRoot(root).Generate(context.Background(), cfg)
	require.NoError(t, err)
	info, err := os.Stat(root)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	fileRoot := filepath.Join(base, "file-root")
	require.NoError(t, os.WriteFile(fileRoot, []byte("not a directory"), 0o600))
	_, err = NewWithRoot(fileRoot).Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_RejectsAncestorSwapBetweenRootCreationAndOpen(t *testing.T) {
	container := t.TempDir()
	parent := filepath.Join(container, "parent")
	root := filepath.Join(parent, "missing-workspace")
	relocated := filepath.Join(container, "relocated-parent")
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(parent, 0o700))

	a := NewWithRoot(root)
	a.rootedRootCreatedHook = func() {
		require.NoError(t, os.Rename(parent, relocated))
		require.NoError(t, os.Symlink(outside, parent))
		require.NoError(t, os.Mkdir(filepath.Join(outside, "missing-workspace"), 0o700))
	}
	cfg := config.DefaultFullConfig("omp-root-swap")
	cfg.Platforms = []string{"omp"}

	_, err := a.Generate(context.Background(), cfg)
	assert.ErrorContains(t, err, "workspace changed while creating")
	assert.NoDirExists(t, filepath.Join(outside, "missing-workspace", ".omp"))
	assert.DirExists(t, filepath.Join(relocated, "missing-workspace"))
}

func TestGenerate_RejectsUnsafeMissingRootCreationEdges(t *testing.T) {
	cfg := config.DefaultFullConfig("omp-root-edge")
	cfg.Platforms = []string{"omp"}

	t.Run("symlink ancestor", func(t *testing.T) {
		container, outside := t.TempDir(), t.TempDir()
		link := filepath.Join(container, "linked-parent")
		require.NoError(t, os.Symlink(outside, link))
		_, err := NewWithRoot(filepath.Join(link, "missing-workspace")).Generate(context.Background(), cfg)
		assert.ErrorContains(t, err, "ancestor must be a real directory")
		assert.NoDirExists(t, filepath.Join(outside, "missing-workspace"))
	})

	t.Run("oversized pathname", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), strings.Repeat("x", 5000))
		_, err := NewWithRoot(root).Generate(context.Background(), cfg)
		assert.ErrorContains(t, err, "inspect OMP workspace root")
	})

	t.Run("created leaf becomes outside symlink", func(t *testing.T) {
		parent, outside := t.TempDir(), t.TempDir()
		root := filepath.Join(parent, "missing-workspace")
		a := NewWithRoot(root)
		a.rootedRootCreatedHook = func() {
			require.NoError(t, os.Remove(root))
			require.NoError(t, os.Symlink(outside, root))
		}
		_, err := a.Generate(context.Background(), cfg)
		assert.ErrorContains(t, err, "open created OMP workspace")
		assert.NoDirExists(t, filepath.Join(outside, ".omp"))
	})
}
