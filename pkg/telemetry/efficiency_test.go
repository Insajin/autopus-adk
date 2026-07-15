package telemetry_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeEfficiency_FailedSpendRemainsInAcceptedTaskDenominator(t *testing.T) {
	runs := []telemetry.AgentRun{
		efficiencyRun("t1", telemetry.StatusPass, 800, 200, 0.02),
		efficiencyRun("t2", telemetry.StatusFail, 700, 200, 0.018),
		efficiencyRun("t3", telemetry.StatusPass, 400, 100, 0.01),
	}

	got := telemetry.SummarizeEfficiency(runs)

	assert.Equal(t, int64(2400), got.RawTokens)
	assert.Equal(t, 2, got.AcceptedTasks)
	require.NotNil(t, got.RawTotalTokensPerAcceptedTask)
	assert.InDelta(t, 1200, *got.RawTotalTokensPerAcceptedTask, 0.001)
	assert.Equal(t, int64(900), got.FailedSpendRawTokens)
	assert.InDelta(t, 1, got.ActualCoverage, 0.001)
	assert.Equal(t, 3, got.UniqueModelCallCount)
}

func TestSummarizeEfficiency_ZeroAcceptedTasksReturnsNullWithReason(t *testing.T) {
	run := efficiencyRun("t1", telemetry.StatusFail, 800, 200, 0.02)

	got := telemetry.SummarizeEfficiency([]telemetry.AgentRun{run})

	assert.Nil(t, got.RawTotalTokensPerAcceptedTask)
	assert.Equal(t, telemetry.EfficiencyReasonZeroAcceptedTasks, got.UnavailableReason)
}

func TestSummarizeEfficiency_RetriesUseDistinctFinalTaskAndKeepEveryAttemptSpend(t *testing.T) {
	failed := efficiencyRun("t1", telemetry.StatusFail, 700, 200, 0.018)
	failed.Attempt = 1
	accepted := efficiencyRun("t1", telemetry.StatusPass, 800, 200, 0.02)
	accepted.Attempt = 2
	accepted.Usage[0].Attempt = 2
	accepted.Usage[0].CallID = "call-t1-retry"

	got := telemetry.SummarizeEfficiency([]telemetry.AgentRun{failed, accepted})

	assert.Equal(t, int64(1900), got.RawTokens)
	assert.Equal(t, 1, got.AcceptedTasks)
	require.NotNil(t, got.RawTotalTokensPerAcceptedTask)
	assert.InDelta(t, 1900, *got.RawTotalTokensPerAcceptedTask, 0.001)
}

func TestSummarizeEfficiency_ConflictingFinalAcceptanceFailsClosed(t *testing.T) {
	pass := efficiencyRun("t1", telemetry.StatusPass, 100, 20, 0.01)
	fail := efficiencyRun("t1", telemetry.StatusFail, 80, 20, 0.01)
	fail.Usage[0].CallID = "call-t1-conflict"

	got := telemetry.SummarizeEfficiency([]telemetry.AgentRun{pass, fail})

	assert.Nil(t, got.RawTotalTokensPerAcceptedTask)
	assert.True(t, got.PromotionBlocked)
	assert.Equal(t, telemetry.EfficiencyReasonAcceptanceConflict, got.UnavailableReason)
}

func TestCompareUsageSpend_CacheChangesCostButNotRawTokens(t *testing.T) {
	cold := efficiencyUsage("run-cold", "call-cold", "task", 1000, 200, 0.020)
	warm := efficiencyUsage("run-warm", "call-warm", "task", 1000, 200, 0.014)
	cached := int64(400)
	warm.CachedInputTokens = &cached

	got := telemetry.CompareUsageSpend([]telemetry.UsageEnvelope{cold}, []telemetry.UsageEnvelope{warm})

	assert.Equal(t, int64(1200), got.ColdRawTotalTokens)
	assert.Equal(t, int64(1200), got.WarmRawTotalTokens)
	assert.InDelta(t, 0, got.RawTokenReductionPct, 0.001)
	require.NotNil(t, got.ActualCostReductionPct)
	assert.InDelta(t, 30, *got.ActualCostReductionPct, 0.001)
}

func TestAgentRun_LegacyJSONAndAdditiveUsageRemainCompatible(t *testing.T) {
	legacy := []byte(`{"agent_name":"executor","status":"PASS","estimated_tokens":321}`)
	var run telemetry.AgentRun
	require.NoError(t, json.Unmarshal(legacy, &run))
	assert.Equal(t, 321, run.EstimatedTokens)
	assert.Empty(t, run.Usage)

	run.TaskID = "task-1"
	run.AcceptanceStatus = telemetry.StatusPass
	run.Usage = []telemetry.UsageEnvelope{efficiencyUsage("run", "call", "task-1", 20, 10, 0.001)}
	encoded, err := json.Marshal(run)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"estimated_tokens":321`)
	assert.Contains(t, string(encoded), `"usage"`)
}

func efficiencyRun(taskID, acceptance string, input, output int64, cost float64) telemetry.AgentRun {
	return telemetry.AgentRun{
		AgentName: "executor", TaskID: taskID, Status: telemetry.StatusPass,
		AcceptanceStatus: acceptance, ToolCalls: 1,
		Usage: []telemetry.UsageEnvelope{efficiencyUsage("run-"+taskID, "call-"+taskID, taskID, input, output, cost)},
	}
}

func efficiencyUsage(runID, callID, taskID string, input, output int64, cost float64) telemetry.UsageEnvelope {
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, TaskID: taskID, Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output, ActualCostUSD: &cost,
	})
}
