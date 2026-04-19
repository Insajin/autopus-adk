package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorktreeManager_Remove_ResolvesSymlinkedPath(t *testing.T) {
	mgr := NewWorktreeManager()
	if !mgr.isGitRepo {
		t.Skip("git repository required")
	}

	realRoot := t.TempDir()
	aliasParent := filepath.Join(t.TempDir(), "alias")
	require.NoError(t, os.Symlink(realRoot, aliasParent))

	aliasPath := filepath.Join(aliasParent, "wt")
	branch := fmt.Sprintf("worktree/remove-symlink-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = exec.Command("git", "branch", "-D", branch).Run()
	})

	require.NoError(t, mgr.addWorktreeWithRetry(t.Context(), aliasPath, branch))
	mgr.paths[aliasPath] = struct{}{}

	require.NoError(t, mgr.Remove(t.Context(), aliasPath))
	assert.Equal(t, 0, mgr.ActiveCount())
	assert.NoDirExists(t, aliasPath)
}
