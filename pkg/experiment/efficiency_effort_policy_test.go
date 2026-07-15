package experiment_test

import (
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateMeasurement_CodexReviewEffortPolicyAcceptsExactRoleBindings(t *testing.T) {
	t.Parallel()

	identity := codexReviewIdentity()
	calls := codexReviewCalls(identity, []roleEffort{
		{role: "reviewer", effort: "xhigh"},
		{role: "reviewer", effort: "xhigh"},
		{role: "reviewer", effort: "xhigh"},
		{role: "security-auditor", effort: "max"},
		{role: "review-consolidator", effort: "xhigh"},
	})

	got := experiment.EvaluateMeasurement(calls, neutralHashes())

	assert.Equal(t, "PASS", got.MeasurementGate)
	assert.Equal(t, "PASS", got.NeutralityGate)
	assert.Equal(t, "sufficient_measurement", got.RolloutDecision)
	assert.InDelta(t, 100, got.ActualUsageCapturePct, 0.001)
}

func TestEvaluateMeasurement_CodexReviewEffortPolicyRejectsSpoofedBindings(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		provider string
		role     string
		effort   string
	}{
		{name: "non codex provider", provider: "claude", role: "reviewer", effort: "xhigh"},
		{name: "missing role", provider: "codex", role: "", effort: "xhigh"},
		{name: "unknown role", provider: "codex", role: "executor", effort: "xhigh"},
		{name: "role whitespace", provider: "codex", role: " reviewer", effort: "xhigh"},
		{name: "reviewer max", provider: "codex", role: "reviewer", effort: "max"},
		{name: "security xhigh", provider: "codex", role: "security-auditor", effort: "xhigh"},
		{name: "consolidator max", provider: "codex", role: "review-consolidator", effort: "max"},
		{name: "effort whitespace", provider: "codex", role: "security-auditor", effort: "max "},
		{name: "unknown effort", provider: "codex", role: "reviewer", effort: "ultra"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			identity := codexReviewIdentity()
			identity.Provider = test.provider
			calls := codexReviewCalls(identity, []roleEffort{{role: test.role, effort: test.effort}})

			got := experiment.EvaluateMeasurement(calls, neutralHashes())

			assert.Equal(t, "BLOCKED", got.MeasurementGate)
			assert.Equal(t, "BLOCKED", got.NeutralityGate)
			assert.Contains(t, got.ReasonCodes, "incompatible_measurement_stratum")
		})
	}
}

func TestEvaluateMeasurement_LiteralEffortPolicyPreservesEqualityBehavior(t *testing.T) {
	t.Parallel()

	identity := codexReviewIdentity()
	identity.EffortPolicy = "xhigh"
	calls := codexReviewCalls(identity, []roleEffort{{role: "legacy-unchecked-role", effort: "xhigh"}})

	got := experiment.EvaluateMeasurement(calls, neutralHashes())

	assert.Equal(t, "PASS", got.MeasurementGate)
}

func TestCompareExpectedTasks_CodexMixedEffortsIncludeEveryFullAndCompactCall(t *testing.T) {
	t.Parallel()

	identity := codexReviewIdentity()
	baseline := codexReviewTrial("low-task", "baseline", "AB", identity, []roleEffort{
		{role: "reviewer", effort: "xhigh"},
		{role: "reviewer", effort: "xhigh"},
		{role: "reviewer", effort: "xhigh"},
		{role: "security-auditor", effort: "max"},
		{role: "review-consolidator", effort: "xhigh"},
	}, 100)
	candidate := codexReviewTrial("low-task", "candidate", "AB", identity, []roleEffort{
		{role: "reviewer", effort: "xhigh"},
		{role: "security-auditor", effort: "max"},
	}, 100)

	got := experiment.CompareExpectedTasks([]experiment.TaskTrial{baseline, candidate}, []string{"low-task"})

	require.Equal(t, []string{"low-task"}, got.PairedTaskIDs)
	assert.True(t, got.ExpectedCorpusComplete)
	assert.Equal(t, int64(500), got.PairedARawTokens)
	assert.Equal(t, int64(200), got.PairedBRawTokens)
	assert.InDelta(t, 60, got.PairedReductionPct, 0.001)
}

func TestCompareExpectedTasks_CodexMixedEffortPolicySupportsHighAndCriticalFullDepth(t *testing.T) {
	t.Parallel()

	identity := codexReviewIdentity()
	full := []roleEffort{
		{role: "reviewer", effort: "xhigh"},
		{role: "reviewer", effort: "xhigh"},
		{role: "reviewer", effort: "xhigh"},
		{role: "security-auditor", effort: "max"},
		{role: "review-consolidator", effort: "xhigh"},
	}
	got := experiment.CompareExpectedTasks([]experiment.TaskTrial{
		codexReviewTrial("high-task", "baseline", "AB", identity, full, 100),
		codexReviewTrial("high-task", "candidate", "AB", identity, full, 80),
		codexReviewTrial("critical-task", "baseline", "BA", identity, full, 100),
		codexReviewTrial("critical-task", "candidate", "BA", identity, full, 80),
	}, []string{"high-task", "critical-task"})

	assert.Equal(t, []string{"critical-task", "high-task"}, got.PairedTaskIDs)
	assert.Equal(t, int64(1000), got.PairedARawTokens)
	assert.Equal(t, int64(800), got.PairedBRawTokens)
	assert.True(t, got.ExpectedCorpusComplete)
}

func TestCompareExpectedTasks_CodexMixedEffortSpoofIsExcluded(t *testing.T) {
	t.Parallel()

	identity := codexReviewIdentity()
	got := experiment.CompareExpectedTasks([]experiment.TaskTrial{
		codexReviewTrial("task", "baseline", "AB", identity, []roleEffort{{role: "reviewer", effort: "xhigh"}}, 100),
		codexReviewTrial("task", "candidate", "AB", identity, []roleEffort{{role: "reviewer", effort: "max"}}, 80),
	}, []string{"task"})

	assert.Empty(t, got.PairedTaskIDs)
	assert.Equal(t, []experiment.ExcludedTask{{TaskID: "task", Reason: "incompatible_stratum"}}, got.ExcludedTasks)
	assert.False(t, got.ExpectedCorpusComplete)
}

type roleEffort struct {
	role   string
	effort string
}

func codexReviewIdentity() experiment.ComparisonIdentity {
	return experiment.ComparisonIdentity{
		Provider: "codex", ProviderVersion: "0.144.1", Model: "gpt-5.6-sol", ModelVersion: "2026-07-12",
		EffortPolicy: experiment.CodexReviewEffortPolicyV1, RiskPolicy: "risk-v1",
		CacheStratum: "cold", ConfigHash: "config-hash",
	}
}

func codexReviewCalls(identity experiment.ComparisonIdentity, bindings []roleEffort) []experiment.CallEvidence {
	calls := make([]experiment.CallEvidence, 0, len(bindings))
	for index, binding := range bindings {
		calls = append(calls, experiment.CallEvidence{
			Identity: identity,
			Usage:    codexReviewUsage(identity, "measurement", index, binding, 100),
		})
	}
	return calls
}

func codexReviewTrial(taskID, arm, order string, identity experiment.ComparisonIdentity, bindings []roleEffort, raw int64) experiment.TaskTrial {
	usage := make([]telemetry.UsageEnvelope, 0, len(bindings))
	for index, binding := range bindings {
		usage = append(usage, codexReviewUsage(identity, taskID+"-"+arm, index, binding, raw))
	}
	return experiment.TaskTrial{
		TaskID: taskID, Arm: arm, PairOrder: order, Identity: identity,
		Runs: []telemetry.AgentRun{{
			AgentName: arm, TaskID: taskID, Attempt: 1,
			Status: telemetry.StatusPass, AcceptanceStatus: telemetry.StatusPass, Usage: usage,
		}},
	}
}

func codexReviewUsage(identity experiment.ComparisonIdentity, runID string, index int, binding roleEffort, raw int64) telemetry.UsageEnvelope {
	zero := int64(0)
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: fmt.Sprintf("call-%02d", index), Source: telemetry.UsageSourceProvider,
		Provider: identity.Provider, ProviderVersion: identity.ProviderVersion,
		Model: identity.Model, ModelVersion: identity.ModelVersion, Effort: binding.effort, Role: binding.role,
		RiskPolicy: identity.RiskPolicy, CacheStratum: identity.CacheStratum, ConfigHash: identity.ConfigHash,
		InputTokensTotal: &raw, OutputTokensTotal: &zero,
	})
}
