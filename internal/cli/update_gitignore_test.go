package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// releaseAddedPatterns are ignore entries a release adds after a project was
// already initialized. omp discovers rules and extensions with a gitignore-aware
// glob, so these must be present in directory form; the platform boundary tests
// own that shape, this file owns their delivery through update.
var releaseAddedPatterns = []string{
	".omp/rules/",
	".omp/agents/",
	".omp/config.yml",
	".omp/extensions/",
}

// TestUpdateCmd_DeliversPatternsAddedAfterInit pins the delivery path for ignore
// patterns introduced by a release. init is the only writer that ever existed, so
// an existing project could not receive a new pattern without re-running init.
func TestUpdateCmd_DeliversPatternsAddedAfterInit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "gitignore-delivery", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	gitignorePath := filepath.Join(dir, ".gitignore")
	rewindGitignore(t, gitignorePath, releaseAddedPatterns)

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--yes"})
	require.NoError(t, updateCmd.Execute(), out.String())

	lines := gitignoreLines(t, gitignorePath)
	for _, pattern := range releaseAddedPatterns {
		assert.True(t, lines[pattern],
			"update must deliver %q; only init wrote ignore patterns before", pattern)
	}
	assert.Contains(t, out.String(), ".gitignore updated",
		"a write this command performed must be reported")
}

// TestUpdateCmd_PreviewReportsGitignoreWithoutWriting keeps the preview honest:
// --plan must name the pending .gitignore work and must not perform it.
func TestUpdateCmd_PreviewReportsGitignoreWithoutWriting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "gitignore-preview", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	gitignorePath := filepath.Join(dir, ".gitignore")
	rewindGitignore(t, gitignorePath, releaseAddedPatterns)
	before, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)

	var out bytes.Buffer
	previewCmd := newTestRootCmd()
	previewCmd.SetOut(&out)
	previewCmd.SetArgs([]string{"update", "--dir", dir, "--plan"})
	require.NoError(t, previewCmd.Execute(), out.String())

	assert.Contains(t, out.String(), ".gitignore would be updated")
	after, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "--plan must not write")
}

// TestUpdateCmd_LeavesSatisfiedGitignoreUntouched keeps the write idempotent:
// a second update must neither duplicate a pattern nor claim it changed anything.
func TestUpdateCmd_LeavesSatisfiedGitignoreUntouched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "gitignore-idempotent", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	gitignorePath := filepath.Join(dir, ".gitignore")
	before, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--yes"})
	require.NoError(t, updateCmd.Execute(), out.String())

	after, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
	assert.NotContains(t, out.String(), ".gitignore updated")
}

// TestUpdateCmd_PreservesUserGitignoreEntries pins that the delivery is additive.
// The file is the user's; update owns only the patterns it appends.
func TestUpdateCmd_PreservesUserGitignoreEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "gitignore-user", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	gitignorePath := filepath.Join(dir, ".gitignore")
	rewindGitignore(t, gitignorePath, releaseAddedPatterns)
	existing, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	const userEntry = "my-secret-scratch/"
	require.NoError(t, os.WriteFile(gitignorePath,
		append(existing, []byte(userEntry+"\n")...), 0o644))

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--yes"})
	require.NoError(t, updateCmd.Execute())

	lines := gitignoreLines(t, gitignorePath)
	assert.True(t, lines[userEntry], "a user entry must survive the update")
	assert.True(t, lines[".omp/rules/"])
}

// rewindGitignore drops patterns from an existing file, reproducing a project
// initialized by a release that predates them.
func rewindGitignore(t *testing.T, path string, drop []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	dropped := make(map[string]bool, len(drop))
	for _, pattern := range drop {
		dropped[pattern] = true
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if dropped[strings.TrimSpace(line)] {
			continue
		}
		kept = append(kept, line)
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644))

	remaining := gitignoreLines(t, path)
	for _, pattern := range drop {
		require.False(t, remaining[pattern],
			"the fixture must actually remove %q", pattern)
	}
}

func gitignoreLines(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines[trimmed] = true
		}
	}
	return lines
}
