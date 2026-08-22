package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextObserveSession_WritesObservedBodyFreePromotionEvidence(t *testing.T) {
	requireDarwinManagedOMPSandboxForTest(t)
	challenge := workflowContextRuntimeHash("observe-session-challenge")
	setup, options, logPath := workflowContextObserveEvidenceFixture(t, challenge)
	input, taskBodies := workflowContextObserveEvidenceInput(t, challenge)
	var output bytes.Buffer
	prepared := &setup
	ctx := context.WithValue(context.Background(), workflowContextObserveSessionPrepareKey{},
		workflowContextObserveSessionPrepare(func(_ context.Context, got workflowContextObserveSessionOptions, gotChallenge string) (workflowContextObserveSessionSetup, error) {
			require.Equal(t, options, got)
			require.Equal(t, challenge, gotChallenge)
			return *prepared, nil
		}))

	require.NoError(t, RunWorkflowContextObserveSession(ctx, input, &output, options))
	responses := decodeWorkflowContextObserveResponses(t, output.Bytes())
	require.Len(t, responses, 42)
	shutdown := responses[len(responses)-1]
	assert.Equal(t, "shutdown", shutdown.Type)
	assert.Equal(t, 40, shutdown.CallsCompleted)
	assert.Equal(t, 18, shutdown.CompactionCycles)
	assert.True(t, shutdown.CleanupVerified)
	assert.NotEmpty(t, shutdown.EvidenceID)
	assert.NotEmpty(t, shutdown.ReportDigest)
	assert.Zero(t, shutdown.OwnedRootsRemaining)
	assert.Zero(t, shutdown.ProcessesRemaining)

	require.Zero(t, prepared.full.PID())
	require.Zero(t, prepared.optimized.PID())
	_, err := os.Lstat(prepared.taskRoot)
	require.ErrorIs(t, err, os.ErrNotExist)

	reportPath := promptlayer.OMPContextPromotionReportPathV1(options.ProjectDir)
	reportBody, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	for _, forbidden := range append([]string{
		prepared.credential, prepared.endpoint, prepared.projectDir, prepared.taskRoot,
		"credential_locator", "assistant_text", "safe assistant output", "/Users/",
	}, taskBodies...) {
		assert.NotContains(t, string(reportBody), forbidden)
	}
	var report promptlayer.OMPContextPromotionReportV1
	require.NoError(t, json.Unmarshal(reportBody, &report))
	assert.Equal(t, shutdown.EvidenceID, report.EvidenceID)
	assert.Equal(t, challenge, report.ChallengeDigest)
	assert.Len(t, report.Tasks, 20)
	assert.Len(t, report.Observations, 40)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.FullProcessStarts)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.OptimizedProcessStarts)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.FullSessionCount)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.OptimizedSessionCount)
	assert.Equal(t, 10, countWorkflowContextObserveOrders(report.Tasks, "AB"))
	assert.Equal(t, 10, countWorkflowContextObserveOrders(report.Tasks, "BA"))
	rows, err := promptlayer.OMPContextPromotionReportCanaryRowsV1(report)
	require.NoError(t, err)
	require.Len(t, rows, 40)
	for index := 0; index < len(rows); index += 2 {
		full, optimized := rows[index], rows[index+1]
		if full.Variant == promptlayer.OMPContextCanaryVariantOptimizedV1 {
			full, optimized = optimized, full
		}
		assert.Equal(t, int64(100), full.Tokens)
		expectedOptimizedTokens := int64(40)
		if (index/2)%workflowContextObserveSessionSegmentPairs == 0 {
			expectedOptimizedTokens = 100
		}
		assert.Equal(t, expectedOptimizedTokens, optimized.Tokens)
		assert.True(t, optimized.FallbackVerified)
		assert.True(t, optimized.RollbackVerified)
	}

	storeBody, err := os.ReadFile(promptlayer.OMPContextEvidenceStorePath(options.ProjectDir))
	require.NoError(t, err)
	assert.NotContains(t, string(storeBody), prepared.credential)
	assert.NotContains(t, string(storeBody), prepared.projectDir)
	var store promptlayer.OMPContextEvidenceStoreV1
	require.NoError(t, json.Unmarshal(storeBody, &store))
	assert.Equal(t, options.WorkspaceID, store.Binding.WorkspaceID)
	assert.Equal(t, prepared.candidate.Snapshot.SnapshotHash, store.Binding.SnapshotHash)
	assert.Len(t, store.CanaryRows, 40)
	assert.Empty(t, store.HistoryRefs)

	verifyWorkflowContextObserveProviderLog(t, logPath, taskBodies)
}

func TestWorkflowContextObserveSession_WritesBodyFreeErrorFrame(t *testing.T) {
	var output bytes.Buffer
	err := RunWorkflowContextObserveSession(
		context.Background(), strings.NewReader("{}\n"), &output, workflowContextObserveSessionOptions{},
	)
	require.ErrorContains(t, err, "handshake is invalid")
	responses := decodeWorkflowContextObserveResponses(t, output.Bytes())
	require.Len(t, responses, 1)
	assert.Equal(t, workflowContextObserveSessionResponseSchema, responses[0].SchemaVersion)
	assert.Equal(t, "error", responses[0].Type)
	assert.Equal(t, "input_invalid", responses[0].ErrorCode)
	assert.Equal(t, "handshake", responses[0].ErrorStage)
	assert.Zero(t, responses[0].FailedSequence)
	assert.NotContains(t, output.String(), err.Error())
	assert.Equal(t, "network_transport",
		workflowContextObserveSessionErrorCode(fmt.Errorf("provider timed out")))
	assert.Equal(t, "runtime_failed", workflowContextObserveSessionErrorCode(
		fmt.Errorf("observe-session call 6 failed closed: managed active OMP transcript image is invalid"),
	))
}

func TestWorkflowContextObserveSessionHelp_DocumentsExplicitLiveLoopbackCanary(t *testing.T) {
	cmd := newWorkflowContextObserveSessionCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, output.String(), "http://127.0.0.1:43123")
	assert.Contains(t, output.String(), "--explicit-live")
	assert.Contains(t, output.String(), "promotion-report-v1.json")
}

func TestWorkflowContextObserveSessionCommand_RequiresExplicitLiveAcknowledgement(t *testing.T) {
	cmd := newWorkflowContextObserveSessionCmd()
	cmd.SetArgs(nil)
	err := cmd.Execute()
	require.ErrorContains(t, err, "requires --explicit-live")
}
