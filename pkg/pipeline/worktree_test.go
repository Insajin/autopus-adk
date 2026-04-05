package pipeline_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// TestWorktreeManager_Create_ReturnsPath verifies that WorktreeManager.Create
// returns a non-empty path for a new worktree.
func TestWorktreeManager_Create_ReturnsPath(t *testing.T) {
	t.Parallel()

	// Given: a WorktreeManager
	mgr := pipeline.NewWorktreeManager()

	// When: Create is called
	path, err := mgr.Create(t.Context(), "phase-plan")

	// Then: a path is returned and the worktree ID is tracked
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Cleanup
	_ = mgr.Remove(t.Context(), path)
}

// TestWorktreeManager_Remove_CleansUp verifies that WorktreeManager.Remove
// cleans up the worktree and removes it from tracking.
func TestWorktreeManager_Remove_CleansUp(t *testing.T) {
	t.Parallel()

	// Given: a WorktreeManager with a created worktree
	mgr := pipeline.NewWorktreeManager()
	path, err := mgr.Create(t.Context(), "phase-remove")
	require.NoError(t, err)
	require.NotEmpty(t, path)

	// When: Remove is called
	removeErr := mgr.Remove(t.Context(), path)

	// Then: no error and the worktree count decreases
	require.NoError(t, removeErr)
	assert.Equal(t, 0, mgr.ActiveCount())
}

// TestWorktreeManager_Create_MaxLimit verifies that WorktreeManager rejects
// creation when the 5-worktree limit is reached.
func TestWorktreeManager_Create_MaxLimit(t *testing.T) {
	t.Parallel()

	// Given: a WorktreeManager already at max capacity (5 worktrees)
	mgr := pipeline.NewWorktreeManager()
	var paths []string
	for i := 0; i < 5; i++ {
		p, err := mgr.Create(t.Context(), "phase-limit")
		require.NoError(t, err, "unexpected error creating worktree %d", i)
		paths = append(paths, p)
	}

	// When: a 6th worktree is requested
	_, err := mgr.Create(t.Context(), "phase-overflow")

	// Then: an error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit")

	// Cleanup
	for _, p := range paths {
		_ = mgr.Remove(t.Context(), p)
	}
}

// TestWorktreeManager_Create_UsesOSTempDir verifies that WorktreeManager
// creates worktrees under os.TempDir().
func TestWorktreeManager_Create_UsesOSTempDir(t *testing.T) {
	t.Parallel()

	// Given: a WorktreeManager
	mgr := pipeline.NewWorktreeManager()

	// When: Create is called
	path, err := mgr.Create(t.Context(), "phase-tempdir")

	// Then: the path is under os.TempDir()
	require.NoError(t, err)
	tmpDir := os.TempDir()
	assert.True(t,
		len(path) > len(tmpDir) && path[:len(tmpDir)] == tmpDir,
		"expected path %q to be under %q", path, tmpDir,
	)

	// Cleanup
	_ = mgr.Remove(t.Context(), path)
}
