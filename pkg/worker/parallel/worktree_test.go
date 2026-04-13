package parallel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realTempDir returns a t.TempDir() with symlinks resolved, which is
// necessary on macOS where /var is a symlink to /private/var and git
// reports the resolved path.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return real
}

func TestWorktreePath_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseDir string
		taskID  string
		want    string
	}{
		{
			name:    "simple task ID",
			baseDir: "/repo",
			taskID:  "task-1",
			want:    filepath.Join("/repo", ".worktrees", "worker-task-1"),
		},
		{
			name:    "complex task ID",
			baseDir: "/home/user/project",
			taskID:  "abc-123-def",
			want:    filepath.Join("/home/user/project", ".worktrees", "worker-abc-123-def"),
		},
		{
			name:    "minimal task ID",
			baseDir: "/tmp",
			taskID:  "a",
			want:    filepath.Join("/tmp", ".worktrees", "worker-a"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewWorktreeManager(tt.baseDir)
			got := m.worktreePath(tt.taskID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorktreeManager_CreateCommandArgs(t *testing.T) {
	t.Parallel()

	// Initialize a real git repo so we can test Create without error.
	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)
	wtPath, err := m.Create("test-task")
	require.NoError(t, err)

	expected := filepath.Join(tmpDir, ".worktrees", "worker-test-task")
	assert.Equal(t, expected, wtPath)

	// Cleanup the worktree.
	require.NoError(t, m.Remove(wtPath, false))
}

func TestWorktreeManager_CreateAddsSymphonyExclude(t *testing.T) {
	t.Parallel()

	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)
	wtPath, err := m.Create("exclude-task")
	require.NoError(t, err)

	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(wtPath, gitDir)
	}
	excludePath := filepath.Join(gitDir, "info", "exclude")
	content, err := os.ReadFile(excludePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), ".symphony/")

	require.NoError(t, m.Remove(wtPath, false))
}

func TestWorktreeManager_RemoveForce(t *testing.T) {
	t.Parallel()

	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)
	wtPath, err := m.Create("force-task")
	require.NoError(t, err)

	// Force remove should succeed.
	require.NoError(t, m.Remove(wtPath, true))
}

func TestWorktreeManager_List(t *testing.T) {
	t.Parallel()

	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)

	// Initially no extra worktrees.
	paths, err := m.List()
	require.NoError(t, err)
	assert.Empty(t, paths)

	// Create two worktrees and verify they appear in List.
	wt1, err := m.Create("list-1")
	require.NoError(t, err)
	wt2, err := m.Create("list-2")
	require.NoError(t, err)

	paths, err = m.List()
	require.NoError(t, err)
	assert.Len(t, paths, 2)
	assert.Contains(t, paths, wt1)
	assert.Contains(t, paths, wt2)

	// Cleanup.
	require.NoError(t, m.Remove(wt1, false))
	require.NoError(t, m.Remove(wt2, false))
}

func TestNewWorktreeManager(t *testing.T) {
	t.Parallel()

	m := NewWorktreeManager("/some/path")
	assert.NotNil(t, m)
	assert.Equal(t, "/some/path", m.baseDir)
}

func TestWorktreeManager_IsCleanAndRemoveIfClean(t *testing.T) {
	t.Parallel()

	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)
	wtPath, err := m.Create("clean-check")
	require.NoError(t, err)

	clean, err := m.IsClean(wtPath)
	require.NoError(t, err)
	assert.True(t, clean)

	removed, err := m.RemoveIfClean(wtPath)
	require.NoError(t, err)
	assert.True(t, removed)
}

func TestWorktreeManager_RemoveIfClean_PreservesDirtyWorktree(t *testing.T) {
	t.Parallel()

	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)
	wtPath, err := m.Create("dirty-check")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "note.txt"), []byte("dirty"), 0o644))

	clean, err := m.IsClean(wtPath)
	require.NoError(t, err)
	assert.False(t, clean)

	removed, err := m.RemoveIfClean(wtPath)
	require.NoError(t, err)
	assert.False(t, removed)

	_, err = os.Stat(wtPath)
	require.NoError(t, err)

	require.NoError(t, m.Remove(wtPath, true))
}

// initGitRepo initializes a bare-minimum git repo with one commit
// so that worktree operations can succeed.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git init cmd %v failed: %s", args, out)
	}
}
