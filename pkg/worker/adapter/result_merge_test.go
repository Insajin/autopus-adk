package adapter

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeSequentialResult_GeminiPlainText(t *testing.T) {
	first := TaskResult{Output: "line one"}
	second := TaskResult{Output: "line two"}

	got := MergeSequentialResult("gemini", first, true, second)

	assert.Equal(t, "line one\nline two", got.Output)
}

func TestMergeSequentialResult_NonGeminiUsesLatestResult(t *testing.T) {
	first := TaskResult{Output: "line one"}
	second := TaskResult{Output: "line two"}

	got := MergeSequentialResult("claude", first, true, second)

	assert.Equal(t, "line two", got.Output)
}

func TestMergeSequentialResult_GeminiStructuredUsesLatestResult(t *testing.T) {
	first := TaskResult{Output: "line one", SessionID: "s1"}
	second := TaskResult{Output: "line two"}

	got := MergeSequentialResult("gemini", first, true, second)

	assert.Equal(t, "line two", got.Output)
}

func TestMergeSequentialResult_LateUsagePreservesPreviousOutput(t *testing.T) {
	first := TaskResult{Output: "final answer", SessionID: "session", Artifacts: []Artifact{{Name: "report"}}, ToolCalls: 1}
	second := TaskResult{Usage: []telemetry.UsageEnvelope{actualUsageForMerge("run", "call", 10, 5)}, ToolCalls: 2}

	got := MergeSequentialResult("codex", first, true, second)

	assert.Equal(t, "final answer", got.Output)
	assert.Equal(t, "session", got.SessionID)
	require.Len(t, got.Artifacts, 1)
	assert.Equal(t, 3, got.ToolCalls)
	require.Len(t, got.Usage, 1)
	assert.Equal(t, int64(15), *telemetry.AggregateUsage(got.Usage).RawTotalTokens)
}

func TestMergeSequentialResult_DeduplicatesAndRetainsUsageConflicts(t *testing.T) {
	original := actualUsageForMerge("run", "call", 10, 5)
	duplicate := actualUsageForMerge("run", "call", 10, 5)
	conflict := actualUsageForMerge("run", "call", 11, 5)

	deduped := MergeSequentialResult("codex", TaskResult{Usage: []telemetry.UsageEnvelope{original}}, true, TaskResult{Usage: []telemetry.UsageEnvelope{duplicate}})
	require.Len(t, deduped.Usage, 1)
	assert.False(t, telemetry.AggregateUsage(deduped.Usage).PromotionBlocked)

	conflicted := MergeSequentialResult("codex", deduped, true, TaskResult{Usage: []telemetry.UsageEnvelope{conflict}})
	require.Len(t, conflicted.Usage, 2)
	aggregate := telemetry.AggregateUsage(conflicted.Usage)
	assert.True(t, aggregate.PromotionBlocked)
	assert.Equal(t, telemetry.UsageReasonDuplicateCallConflict, aggregate.UnavailableReason)
}

func TestMergeSequentialResult_UnboundUsageIsNeverDeduplicated(t *testing.T) {
	usage := actualUsageForMerge("temporary", "temporary", 10, 5)
	usage.RunID = ""
	usage.CallID = ""

	got := MergeSequentialResult("codex", TaskResult{Usage: []telemetry.UsageEnvelope{usage}}, true, TaskResult{Usage: []telemetry.UsageEnvelope{usage}})

	require.Len(t, got.Usage, 2)
}

func actualUsageForMerge(runID, callID string, input, output int64) telemetry.UsageEnvelope {
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, Provider: "codex", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})
}
