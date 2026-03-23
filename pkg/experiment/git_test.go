package experiment

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary git repository for testing.
// Returns the repo path and a cleanup function.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "experiment-git-test-*")
	require.NoError(t, err)

	cmds := [][]string{
		{"git", "-c", "gc.auto=0", "init"},
		{"git", "-c", "gc.auto=0", "config", "user.email", "test@test.com"},
		{"git", "-c", "gc.auto=0", "config", "user.name", "Test"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}

	// Create an initial commit so HEAD exists
	readmeFile := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(readmeFile, []byte("# test\n"), 0644))

	commitCmds := [][]string{
		{"git", "-c", "gc.auto=0", "add", "."},
		{"git", "-c", "gc.auto=0", "commit", "-m", "initial commit"},
	}
	for _, args := range commitCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	return dir, cleanup
}

func TestCreateExperimentBranch(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	g := NewGit(dir)
	err := g.CreateExperimentBranch("test-session-123")
	require.NoError(t, err)

	// Verify we are now on the experiment branch
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "experiment/XLOOP-test-session-123")
}

func TestCommitExperiment(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("sess-456"))

	// Create a new file to commit
	newFile := filepath.Join(dir, "change.txt")
	require.NoError(t, os.WriteFile(newFile, []byte("change\n"), 0644))

	hash, err := g.CommitExperiment(1, "test change")
	require.NoError(t, err)
	assert.NotEmpty(t, hash, "commit hash should not be empty")
	assert.Len(t, hash, 40, "commit hash should be 40 chars")
}

func TestResetToCommit(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("sess-reset"))

	// Get initial HEAD hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	initialHash := string(out[:40])

	// Make a change and commit
	newFile := filepath.Join(dir, "temp.txt")
	require.NoError(t, os.WriteFile(newFile, []byte("temp\n"), 0644))
	_, err = g.CommitExperiment(1, "temp change")
	require.NoError(t, err)

	// Reset back
	err = g.ResetToCommit(initialHash)
	require.NoError(t, err)

	// Verify HEAD is back to initialHash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err = cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, initialHash, string(out[:40]))
}

func TestCheckCleanWorktree(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	g := NewGit(dir)

	// Clean repo should not return error
	err := g.CheckCleanWorktree()
	require.NoError(t, err)

	// Dirty repo should return error
	dirtyFile := filepath.Join(dir, "dirty.txt")
	require.NoError(t, os.WriteFile(dirtyFile, []byte("dirty\n"), 0644))

	// Add but don't commit — still dirty (untracked counts as dirty for our check)
	err = g.CheckCleanWorktree()
	assert.Error(t, err)
}

func TestGetDiffStats(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("sess-diff"))

	// Get base commit
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	baseHash := string(out[:40])

	// Make changes
	newFile := filepath.Join(dir, "newfile.go")
	require.NoError(t, os.WriteFile(newFile, []byte("package main\n\nfunc main() {}\n"), 0644))
	_, err = g.CommitExperiment(1, "add new file")
	require.NoError(t, err)

	added, removed, files, err := g.GetDiffStats(baseHash)
	require.NoError(t, err)
	assert.Greater(t, added, 0, "should have added lines")
	assert.GreaterOrEqual(t, removed, 0)
	assert.Contains(t, files, "newfile.go")
}

func TestIsExperimentBranch(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	g := NewGit(dir)

	// On main/master, should return false
	isExp, err := g.IsExperimentBranch()
	require.NoError(t, err)
	assert.False(t, isExp, "main branch should not be experiment branch")

	// Switch to experiment branch
	require.NoError(t, g.CreateExperimentBranch("sess-check"))

	isExp, err = g.IsExperimentBranch()
	require.NoError(t, err)
	assert.True(t, isExp, "experiment branch should return true")
}
