package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A probe cohort runs the same 40 calls as the evidence producer and must end
// without a report, an evidence store or a signature. Its only product is the
// record file.
func TestWorkflowContextObserveSession_ProbeRecordsCohortWithoutEvidence(t *testing.T) {
	run := runWorkflowContextObserveProbe(t, "model-a", 0)

	require.ErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
	require.Len(t, run.responses, 42)
	last := run.responses[len(run.responses)-1]
	assert.Equal(t, "error", last.Type)
	assert.Equal(t, "probe_completed", last.ErrorCode)
	assert.Equal(t, "probe", last.ErrorStage)
	assert.Empty(t, last.EvidenceID)
	assert.Empty(t, last.ReportDigest)
	assert.Empty(t, last.GateDiagnostic)

	_, err := os.Stat(promptlayer.OMPContextPromotionReportPathV1(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist, "a probe never writes a promotion report")
	_, err = os.Stat(promptlayer.OMPContextEvidenceStorePath(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist, "a probe never writes an evidence store")
	info, err := os.Stat(run.probePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	directory, err := os.Stat(filepath.Dir(run.probePath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), directory.Mode().Perm())

	calls := workflowContextObserveProbeRecordsOfKind(run.records, "call")
	require.Len(t, calls, 40)
	require.Len(t, workflowContextObserveProbeRecordsOfKind(run.records, "compaction"), 18)
	for index, record := range calls {
		assert.Equal(t, index+1, record.Sequence)
		assert.Contains(t, []string{"A", "B"}, record.Variant)
		assert.Contains(t, []int{1, 2}, record.SessionSegment)
		assert.GreaterOrEqual(t, record.SessionSequence, 1)
		assert.LessOrEqual(t, record.SessionSequence, workflowContextObserveSessionSegmentPairs)
		require.NotNil(t, record.StatsBefore, "call %d keeps the billed input it started from", record.Sequence)
		require.NotNil(t, record.StatsAfter)
		require.NotNil(t, record.BranchUsageDelta)
		assert.Equal(t, record.StatsAfter.Total-record.StatsBefore.Total, record.BranchUsageDelta.Total)
		assert.Empty(t, record.AbortReason)
		assert.Zero(t, record.RejectedIdentifiers)
		// The four closed enums belong to compaction records only.
		assert.Empty(t, record.Outcome)
		assert.Empty(t, record.Method)
		assert.Empty(t, record.Refusal)
		assert.Empty(t, record.AttemptCoverage)
	}
	for _, forbidden := range append(
		[]string{"safe assistant output", "safe compacted context", "CANONICAL-AGENT-DOCUMENT"},
		run.taskBodies...,
	) {
		assert.NotContains(t, string(run.body), forbidden,
			"probe records carry metadata, never prompt or assistant content")
	}
}

// Without a maintenance usage source the probe records absence, not zero: a
// completed compaction whose cost is unknown must not read as free.
func TestWorkflowContextObserveSession_ProbeKeepsUnobservedMaintenanceUnknown(t *testing.T) {
	run := runWorkflowContextObserveProbe(t, "model-a", 0)

	require.ErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
	compactions := workflowContextObserveProbeRecordsOfKind(run.records, "compaction")
	require.Len(t, compactions, 18)
	for _, record := range compactions {
		assert.Equal(t, "B", record.Variant)
		assert.Equal(t, "completed", record.Outcome)
		assert.Equal(t, "none", record.Method, "no native frame named a method")
		assert.Equal(t, "none", record.Refusal)
		assert.Equal(t, "unknown", record.AttemptCoverage)
		assert.False(t, record.MaintenanceUsagePresent)
		assert.Nil(t, record.MaintenanceInputTokens)
		assert.Nil(t, record.MaintenanceOutputTokens)
		assert.Nil(t, record.TokensBefore)
		assert.Nil(t, record.TokensAfter)
		assert.Empty(t, record.MaintenanceUsageKeys)
		assert.Equal(t, []string{"confirm"}, record.UIRequestMethods)
		assert.Contains(t, record.Roles, "assistant")
	}
}

// The probe overlay lets a remote compaction run, so the probe can read the
// method, the provider usage keys and the native estimates that decide H1'.
func TestWorkflowContextObserveSession_ProbeObservesRemoteCompactionMetadata(t *testing.T) {
	run := runWorkflowContextObserveProbe(t, pipelineOMPActiveProbeRemoteModel, 0)

	require.ErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
	compactions := workflowContextObserveProbeRecordsOfKind(run.records, "compaction")
	require.Len(t, compactions, 18)
	for _, record := range compactions {
		assert.Equal(t, "completed", record.Outcome)
		assert.Equal(t, "remote", record.Method)
		assert.Equal(t, "unknown", record.AttemptCoverage,
			"final usage alone never proves every provider attempt was billed")
		assert.True(t, record.MaintenanceUsagePresent)
		assert.Equal(t, []string{"inputTokens", "outputTokens", "totalTokens"}, record.MaintenanceUsageKeys)
		require.NotNil(t, record.MaintenanceInputTokens)
		assert.Equal(t, int64(20000), *record.MaintenanceInputTokens)
		require.NotNil(t, record.MaintenanceOutputTokens)
		assert.Equal(t, int64(500), *record.MaintenanceOutputTokens)
		require.NotNil(t, record.TokensBefore)
		assert.Equal(t, int64(61904), *record.TokensBefore)
		require.NotNil(t, record.TokensAfter)
		assert.Equal(t, int64(30000), *record.TokensAfter)
		assert.Equal(t, 1, record.CompactionImages)
		assert.Equal(t, []string{"notify", "confirm"}, record.UIRequestMethods,
			"a notice is recorded by method name and never confirmed")
		assert.Contains(t, record.Roles, "compactionSummary")
		assert.GreaterOrEqual(t, record.ContentTypes["image"], 1)
		assert.Zero(t, record.RejectedIdentifiers)
	}
}

// A cohort where every compaction is declined is a valid measurement. The
// promotion floor of two completed compactions must not turn it into a failure.
func TestWorkflowContextObserveSession_ProbeRecordsDeclinesAndSkipsPromotionFloor(t *testing.T) {
	run := runWorkflowContextObserveProbe(t, pipelineOMPActiveProbeRefuseModel, 0)

	require.ErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
	compactions := workflowContextObserveProbeRecordsOfKind(run.records, "compaction")
	require.Len(t, compactions, 18)
	for _, record := range compactions {
		assert.Equal(t, "refused", record.Outcome)
		assert.Equal(t, "too_small", record.Refusal)
		assert.Equal(t, "none", record.Method)
		assert.Zero(t, record.CompactionImages)
		assert.False(t, record.MaintenanceUsagePresent)
		assert.Empty(t, record.UIRequestMethods)
		assert.Empty(t, record.Roles, "a declined compaction has no post-compaction page to tally")
	}
	for _, response := range run.responses {
		assert.Zero(t, response.CompactionCycles)
	}
}

// An interrupted probe keeps what it observed and names why it stopped, and it
// is never reported as completed.
func TestWorkflowContextObserveSession_PartialProbeRetainsRecordsAndAbortReason(t *testing.T) {
	run := runWorkflowContextObserveProbe(t, "model-a", 5)

	require.Error(t, run.err)
	assert.NotErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
	last := run.responses[len(run.responses)-1]
	assert.Equal(t, "error", last.Type)
	assert.Equal(t, "input_invalid", last.ErrorCode)

	calls := workflowContextObserveProbeRecordsOfKind(run.records, "call")
	require.Len(t, calls, 6)
	for _, record := range calls[:5] {
		assert.Empty(t, record.AbortReason)
		assert.NotEmpty(t, record.Variant)
	}
	abort := calls[5]
	assert.Equal(t, "input_invalid", abort.AbortReason)
	assert.Empty(t, abort.Variant)
	assert.Zero(t, abort.Sequence)
	assert.Len(t, workflowContextObserveProbeRecordsOfKind(run.records, "compaction"), 1)
	_, err := os.Stat(promptlayer.OMPContextPromotionReportPathV1(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
