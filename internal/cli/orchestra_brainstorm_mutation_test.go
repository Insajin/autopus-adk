package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func readBrainstormFailureReport(t *testing.T, repo string) orchestraFailureReport {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repo, ".autopus", "orchestra", "failed-brainstorm-debate-*.json"))
	require.NoError(t, err)
	require.Len(t, matches, 1, "exactly one failure artifact must be written")
	data, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	var report orchestraFailureReport
	require.NoError(t, json.Unmarshal(data, &report))
	assert.Contains(t, string(data), `"changed_files"`)
	return report
}

func TestOrchestraBrainstorm_ProviderWriteFailsRunWithoutRollback(t *testing.T) {
	fixture := newBrainstormFixture(t, true)
	sentinel := filepath.Join(fixture.repo, "sentinel.txt")
	t.Setenv("AUTOPUS_TEST_SENTINEL", sentinel)
	t.Setenv("AUTOPUS_TEST_WRITER", "codex")

	err := fixture.run(t, 30, OrchestraFlags{})

	require.Error(t, err)
	var mutation *orchestra.WorkspaceMutationError
	require.ErrorAs(t, err, &mutation)
	assert.Equal(t, []string{"sentinel.txt"}, mutation.ChangedFiles)
	assert.Nil(t, mutation.Cause, "a clean run has no other failure to attach")
	assert.Contains(t, err.Error(), orchestra.DegradedReasonWorkspaceMutation)
	assert.Contains(t, err.Error(), "sentinel.txt")
	content, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr, "no rollback: the provider's file must survive")
	assert.Equal(t, "mutation\n", string(content))

	require.NotNil(t, fixture.result)
	require.NotNil(t, fixture.result.Workspace)
	assert.True(t, fixture.result.Workspace.MutationDetected)
	assert.Equal(t, orchestra.WorkspaceStatusMutated, fixture.result.Workspace.Status)
	assert.Equal(t, orchestra.TerminalBlocked, fixture.result.TerminalState)
	assert.Contains(t, fixture.result.DegradedReasons, orchestra.DegradedReasonWorkspaceMutation)

	report := readBrainstormFailureReport(t, fixture.repo)
	assert.Contains(t, report.Error, orchestra.DegradedReasonWorkspaceMutation)
	require.NotNil(t, report.RunReceipt)
	require.NotNil(t, report.RunReceipt.Workspace)
	assert.Equal(t, []string{"sentinel.txt"}, report.RunReceipt.Workspace.ChangedFiles)
	assert.Equal(t, orchestra.WorkspaceStatusMutated, report.RunReceipt.Workspace.Status)
	assert.NotEqual(t, report.RunReceipt.Workspace.SnapshotBefore.SHA256, report.RunReceipt.Workspace.SnapshotAfter.SHA256)
	assert.Equal(t, report.RunReceipt.Workspace.SnapshotBefore.Entries+1, report.RunReceipt.Workspace.SnapshotAfter.Entries)
}

func TestOrchestraBrainstorm_MutationOutranksTimeoutButKeepsIt(t *testing.T) {
	fixture := newBrainstormFixture(t, true)
	sentinel := filepath.Join(fixture.repo, "sentinel.txt")
	t.Setenv("AUTOPUS_TEST_SENTINEL", sentinel)
	t.Setenv("AUTOPUS_TEST_WRITER", "codex")
	t.Setenv("AUTOPUS_TEST_SLEEPER", "codex")
	t.Setenv("AUTOPUS_TEST_SLEEP", "30")

	err := fixture.run(t, 2, OrchestraFlags{})

	require.Error(t, err)
	var mutation *orchestra.WorkspaceMutationError
	require.ErrorAs(t, err, &mutation)
	assert.Equal(t, []string{"sentinel.txt"}, mutation.ChangedFiles)
	assert.True(t, strings.HasPrefix(err.Error(), "오케스트레이션 실패: "+orchestra.DegradedReasonWorkspaceMutation), err.Error())
	require.NotNil(t, mutation.Cause, "the quorum failure caused by the timeout must remain attached")
	assert.Contains(t, mutation.Cause.Error(), "usable 1/2, required 2")

	require.NotNil(t, fixture.result)
	require.Len(t, fixture.result.FailedProviders, 1)
	assert.Equal(t, "codex", fixture.result.FailedProviders[0].Name)
	assert.True(t, fixture.result.FailedProviders[0].TimedOut)
	assert.False(t, fixture.result.QuorumMet)
	assert.Equal(t, 1, fixture.result.QuorumUsable)

	report := readBrainstormFailureReport(t, fixture.repo)
	require.Len(t, report.FailedProviders, 1)
	assert.Equal(t, "timeout", report.FailedProviders[0].FailureClass)
	require.NotNil(t, report.RunReceipt)
	assert.True(t, report.RunReceipt.Workspace.MutationDetected)
	assert.Contains(t, report.RunReceipt.DegradedReasons, "provider_quorum")
	assert.Contains(t, report.RunReceipt.DegradedReasons, orchestra.DegradedReasonWorkspaceMutation)
	codex := fixture.providerReceipt(t, "codex")
	assert.True(t, codex.TimedOut)
	assert.Greater(t, codex.PID, 0, "the timed-out process must still be attributable")
	assert.Equal(t, fixture.captured.ProviderWorkDir, codex.Cwd)
	assert.Equal(t, orchestra.SandboxModeReadOnly, codex.SandboxMode)
	_, statErr := os.Stat(sentinel)
	assert.NoError(t, statErr, "no rollback after a timeout either")
}
