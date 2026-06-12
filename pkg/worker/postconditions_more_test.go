package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyExecutionPostconditions_CommitAdvancedPasses(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	baseline := captureExecutionBaseline(repo)
	require.True(t, baseline.GitRepo)

	require.NoError(t, os.WriteFile(filepath.Join(repo, "change.txt"), []byte("x"), 0o644))
	runGit(t, repo, "add", "change.txt")
	runGit(t, repo, "commit", "-m", "advance head")

	artifact, err := verifyExecutionPostconditions(repo, "please commit the changes", baseline)
	require.NoError(t, err)
	assert.Equal(t, "postconditions.json", artifact.Name)
	assert.Contains(t, artifact.Data, `"name":"commit"`)
	assert.Contains(t, artifact.Data, `"status":"passed"`)
}

func TestVerifyExecutionPostconditions_CommitNotAdvancedFails(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	baseline := captureExecutionBaseline(repo)

	// No new commit -> HEAD did not advance.
	artifact, err := verifyExecutionPostconditions(repo, "remember to commit your work", baseline)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD did not advance")
	assert.Contains(t, artifact.Data, `"status":"failed"`)
}

func TestVerifyExecutionPostconditions_BranchExistsPasses(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	baseline := captureExecutionBaseline(repo)

	runGit(t, repo, "checkout", "-b", "feature/login")

	artifact, err := verifyExecutionPostconditions(repo, "create branch feature/login", baseline)
	require.NoError(t, err)
	assert.Contains(t, artifact.Data, `"name":"branch"`)
	assert.Contains(t, artifact.Data, `"status":"passed"`)
}

func TestVerifyExecutionPostconditions_BranchMissingFails(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	baseline := captureExecutionBaseline(repo)

	// Reference a branch that was never created.
	artifact, err := verifyExecutionPostconditions(repo, "create branch feature/never-made", baseline)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch")
	assert.Contains(t, artifact.Data, `"status":"failed"`)
}

func TestVerifyExecutionPostconditions_NoBaselineSkips(t *testing.T) {
	t.Parallel()

	// A non-git directory yields an empty baseline; verification is a no-op.
	dir := t.TempDir()
	baseline := captureExecutionBaseline(dir)
	assert.False(t, baseline.GitRepo)

	artifact, err := verifyExecutionPostconditions(dir, "commit and push", baseline)
	require.NoError(t, err)
	assert.Empty(t, artifact.Name)
}

func TestBranchesForVerification_ExplicitBranchesWin(t *testing.T) {
	t.Parallel()

	reqs := taskPostconditions{Branches: []string{"release/v1", "hotfix/x"}}
	got := branchesForVerification(reqs, t.TempDir())
	assert.Equal(t, []string{"release/v1", "hotfix/x"}, got)
}

func TestBranchesForVerification_FallsBackToCurrentBranch(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	runGit(t, repo, "checkout", "-b", "feature/topic")

	got := branchesForVerification(taskPostconditions{}, repo)
	assert.Equal(t, []string{"feature/topic"}, got)
}

func TestBranchesForVerification_SkipsWorkerBranches(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	runGit(t, repo, "checkout", "-b", "worker-abc123")

	// Worker-managed branches are excluded from postcondition verification.
	got := branchesForVerification(taskPostconditions{}, repo)
	assert.Nil(t, got)
}

func TestBranchesForVerification_NonGitDirReturnsNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, branchesForVerification(taskPostconditions{}, t.TempDir()))
}

func TestSanitizeBranchName_RejectsTraversalAndFlags(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", sanitizeBranchName(""))
	assert.Equal(t, "", sanitizeBranchName("  feature/..hack  "))
	assert.Equal(t, "", sanitizeBranchName("-rf"))
	assert.Equal(t, "feature/x", sanitizeBranchName(`"feature/x".`))
}

func TestDetectTaskPostconditions_DetectsAllIntents(t *testing.T) {
	t.Parallel()

	reqs := detectTaskPostconditions("commit the change, create a branch release/2.0 and git push to origin")
	assert.True(t, reqs.CommitRequired)
	assert.True(t, reqs.PushRequired)
	assert.True(t, reqs.BranchRequired)
	assert.Contains(t, strings.Join(reqs.Branches, ","), "release/2.0")
}
