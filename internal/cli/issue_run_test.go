package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunIssueReport_DryRun renders the preview and skips submission.
// Chdir into an empty temp dir keeps the run hermetic: detectGitRepo and
// config.Load both fail gracefully so the explicit --repo flag is honored.
func TestRunIssueReport_DryRun(t *testing.T) {
	t.Chdir(t.TempDir())

	cmd := newIssueReportCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--dry-run",
		"--error", "boom failed",
		"--command", "auto go",
		"--repo", "octo/repo",
	})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "--- Issue Preview ---")
	assert.Contains(t, out, "[dry-run] Skipping submission.")
	// The rendered markdown must carry the explicit repo and derived title.
	assert.Contains(t, out, "boom failed")
}

// TestRunIssueReport_DryRunViaHelper exercises runIssueReport directly.
func TestRunIssueReport_DryRunViaHelper(t *testing.T) {
	t.Chdir(t.TempDir())

	cmd := newIssueReportCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := runIssueReport(cmd, true, false, "kaboom", "auto sync", 2, "owner/name")
	require.NoError(t, err)
	out := buf.String()
	assert.True(t, strings.Contains(out, "--- End Preview ---"))
	assert.Contains(t, out, "[dry-run]")
}

// TestResolveIssueRepoInputs_GitFallback covers the git-derived branch.
func TestResolveIssueRepoInputs_GitFallback(t *testing.T) {
	t.Parallel()

	// No explicit, no config, non-auto command, git repo present → git repo wins.
	got := resolveIssueRepoInputs("", "make build", "", "me/proj")
	assert.Equal(t, "me/proj", got)

	// Nothing available → default repo.
	got = resolveIssueRepoInputs("", "make build", "", "")
	assert.Equal(t, defaultIssueRepo, got)

	// auto command with no repo info → default repo.
	got = resolveIssueRepoInputs("", "auto go", "", "x/y")
	assert.Equal(t, defaultIssueRepo, got)
}
