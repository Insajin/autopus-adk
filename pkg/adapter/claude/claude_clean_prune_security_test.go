package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClean_RejectsSymlinkedEmptyPruneRootBeforeMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude"), 0o755))
	outside := t.TempDir()
	externalTree := filepath.Join(outside, "autopus", "nested")
	require.NoError(t, os.MkdirAll(externalTree, 0o755))
	if err := os.Symlink(outside, filepath.Join(root, ".claude", "rules")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := NewWithRoot(root).cleanManagedSurfaces()

	require.Error(t, err)
	assert.DirExists(t, externalTree)
	assert.DirExists(t, filepath.Join(outside, "autopus"))
}
