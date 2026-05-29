package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeGitDiff_ReturnsSlice deterministically exercises the success path of
// analyzeGitDiff using a hermetic temp git repo. Two commits guarantee HEAD~1 exists
// regardless of the host's checkout depth, so the changed-file parsing loop is always
// covered (avoids coverage drift between full-history local and shallow CI checkouts).
func TestAnalyzeGitDiff_ReturnsSlice(t *testing.T) {
	// Not parallel: uses os.Chdir.
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.test",
		)
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	runGit("init")
	require.NoError(t, os.WriteFile("a.txt", []byte("v1\n"), 0o644))
	runGit("add", "a.txt")
	runGit("commit", "-m", "c1")
	require.NoError(t, os.WriteFile("a.txt", []byte("v2\n"), 0o644))
	runGit("add", "a.txt")
	runGit("commit", "-m", "c2")

	files, err := analyzeGitDiff()
	require.NoError(t, err)
	assert.Contains(t, files, "a.txt")
}

// TestAnalyzeGitDiff_NonGitDir deterministically exercises the error path by running
// in a non-git temp directory.
func TestAnalyzeGitDiff_NonGitDir(t *testing.T) {
	// Not parallel: uses os.Chdir.
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	_, err = analyzeGitDiff()
	assert.Error(t, err, "analyzeGitDiff must error outside a git repository")
}

// TestRunPlaywright_FailsWithoutNpx deterministically exercises the error path by
// emptying PATH so npx is never found, regardless of the host environment.
func TestRunPlaywright_FailsWithoutNpx(t *testing.T) {
	// Not parallel: mutates PATH via t.Setenv.
	t.Setenv("PATH", t.TempDir())

	// npx is absent, so CombinedOutput fails before producing output (out is nil).
	// We only assert on the error path, which is what makes coverage deterministic.
	_, err := runPlaywright("desktop")
	assert.Error(t, err, "runPlaywright must error when npx is absent")
}
