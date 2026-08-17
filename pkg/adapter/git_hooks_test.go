package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realRepoRoot builds a root whose .git is a plausible gitdir (HEAD present).
func realRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644,
	))
	return root
}

// fabricatedGitRoot builds a root whose .git directory exists but carries no
// gitdir markers — the shape a prior hook install leaves behind in a workspace
// that is not a repository.
func fabricatedGitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755))
	return root
}

// linkedWorktreeRoot builds a root whose .git is a gitdir pointer file.
func linkedWorktreeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, ".git"), []byte("gitdir: /elsewhere/.git/worktrees/wt\n"), 0o644,
	))
	return root
}

func TestSupportsRootGitHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root func(*testing.T) string
		want bool
	}{
		{
			// Writing .git/hooks/* here would MkdirAll a .git directory and
			// fabricate a repository marker that git never honors.
			name: "absent .git is not a repository",
			root: func(t *testing.T) string { t.Helper(); return t.TempDir() },
			want: false,
		},
		{
			name: "linked worktree gitdir file",
			root: linkedWorktreeRoot,
			want: false,
		},
		{
			name: "real gitdir with HEAD",
			root: realRepoRoot,
			want: true,
		},
		{
			// A .git directory without HEAD is the residue of the bug, not a repo.
			name: "fabricated .git directory without HEAD",
			root: fabricatedGitRoot,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SupportsRootGitHooks(tt.root(t)))
		})
	}
}

func TestFilterUnsupportedRootGitHookFiles_DropsHooksOutsideRealRepos(t *testing.T) {
	t.Parallel()

	files := []FileMapping{
		{TargetPath: ".codex/config.toml", Checksum: "a", OverwritePolicy: OverwriteMerge},
		{TargetPath: ".git/hooks/pre-commit", Checksum: "b", OverwritePolicy: OverwriteAlways},
		{TargetPath: ".git/hooks/commit-msg", Checksum: "c", OverwritePolicy: OverwriteAlways},
	}

	t.Run("real repo keeps git hooks", func(t *testing.T) {
		t.Parallel()
		got := FilterUnsupportedRootGitHookFiles(realRepoRoot(t), append([]FileMapping(nil), files...))
		assert.Len(t, got, 3)
	})

	t.Run("non-repo root drops git hooks", func(t *testing.T) {
		t.Parallel()
		got := FilterUnsupportedRootGitHookFiles(t.TempDir(), append([]FileMapping(nil), files...))
		require.Len(t, got, 1)
		assert.Equal(t, ".codex/config.toml", got[0].TargetPath)
	})

	t.Run("fabricated .git drops git hooks", func(t *testing.T) {
		t.Parallel()
		got := FilterUnsupportedRootGitHookFiles(fabricatedGitRoot(t), append([]FileMapping(nil), files...))
		require.Len(t, got, 1)
		assert.Equal(t, ".codex/config.toml", got[0].TargetPath)
	})
}

func TestFilterUnsupportedRootGitHookRemoves_DropsHooksOutsideRealRepos(t *testing.T) {
	t.Parallel()

	removes := []TransactionRemove{
		{Path: ".codex/stale.md"},
		{Path: ".git/hooks/pre-commit"},
	}

	t.Run("real repo keeps git hook removes", func(t *testing.T) {
		t.Parallel()
		got := FilterUnsupportedRootGitHookRemoves(realRepoRoot(t), append([]TransactionRemove(nil), removes...))
		assert.Len(t, got, 2)
	})

	t.Run("non-repo root drops git hook removes", func(t *testing.T) {
		t.Parallel()
		got := FilterUnsupportedRootGitHookRemoves(t.TempDir(), append([]TransactionRemove(nil), removes...))
		require.Len(t, got, 1)
		assert.Equal(t, ".codex/stale.md", got[0].Path)
	})
}
