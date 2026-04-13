package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyExecutionPostconditions_PushBranchMissingFails(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	baseline := captureExecutionBaseline(repo)

	require.NoError(t, os.WriteFile(filepath.Join(repo, "note.txt"), []byte("change"), 0o644))
	runGit(t, repo, "checkout", "-b", "autopus-canary-missing")
	runGit(t, repo, "add", "note.txt")
	runGit(t, repo, "commit", "-m", "change")

	artifact, err := verifyExecutionPostconditions(repo, "create branch autopus-canary-missing, commit changes, and git push origin autopus-canary-missing", baseline)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote branch")
	assert.Equal(t, "postconditions.json", artifact.Name)
}

func TestVerifyExecutionPostconditions_PushBranchExistsPasses(t *testing.T) {
	t.Parallel()

	repo := initGitRepoWithOrigin(t)
	baseline := captureExecutionBaseline(repo)

	require.NoError(t, os.WriteFile(filepath.Join(repo, "note.txt"), []byte("change"), 0o644))
	runGit(t, repo, "checkout", "-b", "autopus-canary-pass")
	runGit(t, repo, "add", "note.txt")
	runGit(t, repo, "commit", "-m", "change")
	runGit(t, repo, "push", "-u", "origin", "autopus-canary-pass")

	artifact, err := verifyExecutionPostconditions(repo, "create branch autopus-canary-pass, commit changes, and git push origin autopus-canary-pass", baseline)
	require.NoError(t, err)
	assert.Equal(t, "postconditions.json", artifact.Name)
	assert.Contains(t, artifact.Data, "\"status\":\"passed\"")
}

func TestDetectTaskPostconditions_IgnoresFillerWords(t *testing.T) {
	t.Parallel()

	reqs := detectTaskPostconditions("create and switch to a new branch named autopus-canary-20260414-072233 and push the branch to origin")
	assert.Equal(t, []string{"autopus-canary-20260414-072233"}, reqs.Branches)
}

func initGitRepoWithOrigin(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	bare := t.TempDir()

	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@test.com")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "commit", "--allow-empty", "-m", "init")
	runGit(t, bare, "init", "--bare")
	runGit(t, repo, "remote", "add", "origin", bare)
	defaultBranch := strings.TrimSpace(gitOutputForTest(t, repo, "branch", "--show-current"))
	runGit(t, repo, "push", "-u", "origin", defaultBranch)

	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
}

func gitOutputForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
	return string(out)
}
