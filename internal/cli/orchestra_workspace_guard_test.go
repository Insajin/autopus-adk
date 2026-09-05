package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestWorkspaceGuard_DetectsOnlyChangesMadeDuringTheRun(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initSyncRepo(t, repo)
	syncWrite(t, repo, "wip.go", "already dirty before the run\n")
	syncWrite(t, repo, "stale.txt", "untracked and later removed\n")

	guard := newWorkspaceGuard(repo)
	require.NotNil(t, guard.before)
	assert.Equal(t, mustResolvePath(t, repo), mustResolvePath(t, guard.root))

	// Pre-existing WIP is not a mutation; only the delta counts.
	clean := guard.compare()
	assert.Equal(t, orchestra.WorkspaceStatusClean, clean.Status)
	assert.False(t, clean.MutationDetected)
	assert.Empty(t, clean.ChangedFiles)
	assert.Equal(t, 2, clean.SnapshotBefore.Entries)
	assert.Equal(t, clean.SnapshotBefore.SHA256, clean.SnapshotAfter.SHA256)

	syncWrite(t, repo, "sentinel.txt", "new file\n")                // new untracked
	syncWrite(t, repo, ".seed", "tracked file modified\n")          // tracked → modified
	require.NoError(t, os.Remove(filepath.Join(repo, "stale.txt"))) // untracked → removed
	syncGit(t, repo, "add", "wip.go")                               // untracked → staged
	syncWrite(t, repo, ".autopus/orchestra/prompts/p.md", "x\n")    // orchestrator artifact

	mutated := guard.compare()
	assert.Equal(t, orchestra.WorkspaceStatusMutated, mutated.Status)
	assert.True(t, mutated.MutationDetected)
	assert.Equal(t, []string{".seed", "sentinel.txt", "stale.txt", "wip.go"}, mutated.ChangedFiles)
	assert.NotEqual(t, mutated.SnapshotBefore.SHA256, mutated.SnapshotAfter.SHA256)
}

func TestWorkspaceGuard_SubdirectoryRunExcludesItsArtifactDir(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initSyncRepo(t, repo)
	sub := filepath.Join(repo, "module")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	guard := newWorkspaceGuard(sub)
	require.NotNil(t, guard.before)
	assert.Equal(t, "module/.autopus/orchestra/", guard.exclude)

	syncWrite(t, repo, "module/.autopus/orchestra/result.md", "artifact\n")
	syncWrite(t, repo, ".autopus/orchestra/other.md", "not this run's artifact root\n")

	evidence := guard.compare()
	assert.Equal(t, []string{".autopus/orchestra/other.md"}, evidence.ChangedFiles)
}

func TestWorkspaceGuard_OutsideGitIsUnavailable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	guard := newWorkspaceGuard(dir)
	assert.Nil(t, guard.before)

	syncWrite(t, dir, "sentinel.txt", "unobservable\n")
	evidence := guard.compare()
	assert.Equal(t, orchestra.WorkspaceStatusUnavailable, evidence.Status)
	assert.False(t, evidence.MutationDetected)
	assert.Nil(t, evidence.SnapshotBefore)
	assert.Nil(t, evidence.SnapshotAfter)
	assert.Equal(t, dir, evidence.Root)
}

func TestPorcelainDelta_StatusChangeOnSamePathCounts(t *testing.T) {
	t.Parallel()

	before := []dirtyFile{{Rel: "a.go", Unstaged: true}, {Rel: "b.go", Staged: true}}
	after := []dirtyFile{{Rel: "a.go", Staged: true, Unstaged: true}, {Rel: "b.go", Staged: true}}
	assert.Equal(t, []string{"a.go"}, porcelainDelta(before, after, ".autopus/orchestra/"))
	assert.Empty(t, porcelainDelta(before, before, ""))
}
