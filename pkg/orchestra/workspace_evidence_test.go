package orchestra

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func finalizedDebateResultForWorkspace(t *testing.T) *OrchestraResult {
	t.Helper()
	result := finalizeOrchestrationContract(&OrchestraResult{
		Strategy: StrategyDebate, ConfiguredProviders: []string{"codex", "gemini"},
		RoundHistory: [][]ProviderResponse{{
			{Provider: "codex", Output: "a", ExecutedBackend: "subprocess"},
			{Provider: "gemini", Output: "b", ExecutedBackend: "subprocess"},
		}},
		Responses: []ProviderResponse{
			{Provider: "codex", Output: "a", ExecutedBackend: "subprocess"},
			{Provider: "gemini", Output: "b", ExecutedBackend: "subprocess"},
		},
	})
	require.NotNil(t, result.RunReceipt)
	require.Equal(t, TerminalCompleted, result.TerminalState)
	return result
}

func TestApplyWorkspaceEvidence_MutationBlocksRunAndKeepsPriorTransition(t *testing.T) {
	t.Parallel()

	result := finalizedDebateResultForWorkspace(t)
	ApplyWorkspaceEvidence(result, WorkspaceEvidence{
		Root:             "/repo",
		SnapshotBefore:   &WorkspaceSnapshot{Entries: 0, SHA256: "before"},
		SnapshotAfter:    &WorkspaceSnapshot{Entries: 1, SHA256: "after"},
		MutationDetected: true,
		ChangedFiles:     []string{"sentinel.txt"},
		Status:           WorkspaceStatusMutated,
	})

	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Equal(t, "blocked", result.GateStatus)
	assert.Equal(t, "fail", result.AnalysisVerdict)
	assert.True(t, result.Degraded)
	assert.Contains(t, result.DegradedReasons, DegradedReasonWorkspaceMutation)

	receipt := result.RunReceipt
	require.NotNil(t, receipt.Workspace)
	assert.Equal(t, WorkspaceStatusMutated, receipt.Workspace.Status)
	assert.Equal(t, []string{"sentinel.txt"}, receipt.Workspace.ChangedFiles)
	assert.Equal(t, "before", receipt.Workspace.SnapshotBefore.SHA256)
	assert.Equal(t, 1, receipt.Workspace.SnapshotAfter.Entries)
	assert.Contains(t, receipt.DegradedReasons, DegradedReasonWorkspaceMutation)
	require.Len(t, receipt.Transitions, 2, "the original completed outcome must stay visible before the block")
	assert.Equal(t, TerminalCompleted, receipt.Transitions[0].State)
	assert.Equal(t, TerminalBlocked, receipt.Transitions[1].State)
	assert.Equal(t, 2, receipt.Transitions[1].Sequence)

	data, err := json.Marshal(receipt)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	workspace, ok := decoded["workspace"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, workspace["mutation_detected"])
	assert.Contains(t, workspace, "changed_files")
}

func TestApplyWorkspaceEvidence_CleanLeavesOutcomeUntouched(t *testing.T) {
	t.Parallel()

	result := finalizedDebateResultForWorkspace(t)
	ApplyWorkspaceEvidence(result, WorkspaceEvidence{
		Root: "/repo", Status: WorkspaceStatusClean,
		SnapshotBefore: &WorkspaceSnapshot{SHA256: "same"}, SnapshotAfter: &WorkspaceSnapshot{SHA256: "same"},
	})

	assert.Equal(t, TerminalCompleted, result.TerminalState)
	assert.NotContains(t, result.DegradedReasons, DegradedReasonWorkspaceMutation)
	require.NotNil(t, result.RunReceipt.Workspace)
	assert.Equal(t, WorkspaceStatusClean, result.RunReceipt.Workspace.Status)
	assert.False(t, result.RunReceipt.Workspace.MutationDetected)
	require.Len(t, result.RunReceipt.Transitions, 1)
}

func TestApplyWorkspaceEvidence_UnfinalizedResultOnlyRecordsEvidence(t *testing.T) {
	t.Parallel()

	result := &OrchestraResult{Merged: "advisory"}
	ApplyWorkspaceEvidence(result, WorkspaceEvidence{Root: "/tmp/not-a-repo", Status: WorkspaceStatusUnavailable})

	require.NotNil(t, result.Workspace)
	assert.Equal(t, WorkspaceStatusUnavailable, result.Workspace.Status)
	assert.Nil(t, result.RunReceipt, "evidence must not fabricate a typed receipt for an unfinalized result")
}

func TestReceiptWithoutWorkspaceOmitsBlock(t *testing.T) {
	t.Parallel()

	result := finalizedDebateResultForWorkspace(t)
	data, err := json.Marshal(result.RunReceipt)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"workspace"`)
	assert.NotContains(t, string(data), `"cwd"`)
	assert.Contains(t, string(data), `"quorum_usable":2`)
}

func TestWorkspaceMutationError_SurfacesFilesAndUnwrapsCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("timeout: codex exceeded 30s deadline")
	err := &WorkspaceMutationError{ChangedFiles: []string{"sentinel.txt", "pkg/a.go"}, Cause: cause}

	message := err.Error()
	assert.True(t, len(message) > 0 && message[:len(DegradedReasonWorkspaceMutation)] == DegradedReasonWorkspaceMutation,
		"mutation must lead the surfaced error: %s", message)
	assert.Contains(t, message, "sentinel.txt, pkg/a.go")
	assert.Contains(t, message, "original failure: timeout: codex exceeded 30s deadline")
	assert.ErrorIs(t, err, cause)

	assert.NotContains(t, (&WorkspaceMutationError{ChangedFiles: []string{"x"}}).Error(), "original failure")
}
