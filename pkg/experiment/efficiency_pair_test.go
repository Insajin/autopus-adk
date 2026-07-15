package experiment_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareCompatibleTasks_S20ExactPairedEvidence(t *testing.T) {
	t.Parallel()

	trials := []experiment.TaskTrial{
		acceptedTrial("t3", "baseline", "AB", canonicalIdentity(), 900),
		acceptedTrial("t2", "candidate", "BA", canonicalIdentity(), 1400),
		acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 1000),
		acceptedTrial("t2", "baseline", "BA", canonicalIdentity(), 2000),
		acceptedTrial("t1", "candidate", "AB", canonicalIdentity(), 700),
	}

	got := experiment.CompareCompatibleTasks(trials)

	assert.Equal(t, []string{"t1", "t2"}, got.PairedTaskIDs)
	assert.Equal(t, []string{"t3"}, got.UnpairedTaskIDs)
	assert.Equal(t, int64(3000), got.PairedARawTokens)
	assert.Equal(t, int64(2100), got.PairedBRawTokens)
	assert.InDelta(t, 30, got.PairedReductionPct, 0.001)
	assert.InDelta(t, 30, got.MedianPairedRawReductionPct, 0.001)
	assert.Equal(t, "PASS", got.Provisional25PctTarget)
	assert.Equal(t, 2, got.PairedTaskCount)
	assert.Equal(t, []string{"t1"}, got.ABTaskIDs)
	assert.Equal(t, []string{"t2"}, got.BATaskIDs)
}

func TestCompareCompatibleTasks_IncompatibleStrataAreExcluded(t *testing.T) {
	t.Parallel()

	other := canonicalIdentity()
	other.ModelVersion = "different-model-version"
	got := experiment.CompareCompatibleTasks([]experiment.TaskTrial{
		acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 1000),
		acceptedTrial("t1", "candidate", "AB", other, 700),
	})

	assert.Empty(t, got.PairedTaskIDs)
	require.Len(t, got.ExcludedTasks, 1)
	assert.Equal(t, "t1", got.ExcludedTasks[0].TaskID)
	assert.Equal(t, "incompatible_stratum", got.ExcludedTasks[0].Reason)
}

func TestCompareCompatibleTasks_IncludesRetriesAndTimeoutSpendOncePerTask(t *testing.T) {
	t.Parallel()

	baseline := acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 1000)
	baseline.Runs = []telemetry.AgentRun{
		attemptRun("t1", 1, telemetry.StatusFail, "", 600, "failed"),
		attemptRun("t1", 2, telemetry.StatusFail, "", 400, "timeout"),
		attemptRun("t1", 3, telemetry.StatusPass, telemetry.StatusPass, 1000, "accepted"),
	}
	candidate := acceptedTrial("t1", "candidate", "AB", canonicalIdentity(), 1400)

	got := experiment.CompareCompatibleTasks([]experiment.TaskTrial{baseline, candidate})

	assert.Equal(t, 1, got.PairedTaskCount, "agent attempts must not become samples")
	assert.Equal(t, int64(2000), got.PairedARawTokens)
	assert.Equal(t, int64(1400), got.PairedBRawTokens)
	assert.InDelta(t, 30, got.PairedReductionPct, 0.001)
}

func TestCompareCompatibleTasks_EnvelopeProvenanceMismatchIsExcluded(t *testing.T) {
	t.Parallel()

	for _, mutate := range []func(*telemetry.UsageEnvelope){
		func(v *telemetry.UsageEnvelope) { v.ProviderVersion = "spoof" },
		func(v *telemetry.UsageEnvelope) { v.ModelVersion = "" },
		func(v *telemetry.UsageEnvelope) { v.RiskPolicy = "spoof" },
		func(v *telemetry.UsageEnvelope) { v.CacheStratum = "" },
		func(v *telemetry.UsageEnvelope) { v.ConfigHash = "spoof" },
	} {
		baseline := acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 1000)
		candidate := acceptedTrial("t1", "candidate", "AB", canonicalIdentity(), 700)
		mutate(&candidate.Runs[0].Usage[0])
		got := experiment.CompareCompatibleTasks([]experiment.TaskTrial{baseline, candidate})
		assert.Empty(t, got.PairedTaskIDs)
		require.Len(t, got.ExcludedTasks, 1)
		assert.Equal(t, "incompatible_stratum", got.ExcludedTasks[0].Reason)
	}
}

func TestCompareExpectedTasks_PartialCorpusRetainsEveryExpectedTask(t *testing.T) {
	t.Parallel()

	other := canonicalIdentity()
	other.ModelVersion = "different-model-version"
	unaccepted := acceptedTrial("t3", "candidate", "AB", canonicalIdentity(), 700)
	unaccepted.Runs[0].AcceptanceStatus = telemetry.StatusFail
	got := experiment.CompareExpectedTasks([]experiment.TaskTrial{
		acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 1000),
		acceptedTrial("t1", "candidate", "AB", canonicalIdentity(), 700),
		acceptedTrial("t2", "baseline", "BA", canonicalIdentity(), 1000),
		acceptedTrial("t3", "baseline", "AB", canonicalIdentity(), 1000),
		unaccepted,
		acceptedTrial("t4", "baseline", "BA", canonicalIdentity(), 1000),
		acceptedTrial("t4", "candidate", "BA", other, 700),
	}, []string{"t4", "t2", "t1", "t3"})

	assert.Equal(t, []string{"t1", "t2", "t3", "t4"}, got.ExpectedTaskIDs)
	assert.Equal(t, 4, got.ExpectedTaskCount)
	assert.Equal(t, 1, got.PairedExpectedTaskCount)
	assert.False(t, got.ExpectedCorpusComplete)
	assert.Equal(t, []string{"t1"}, got.PairedTaskIDs)
	assert.Equal(t, []string{"t2"}, got.UnpairedTaskIDs)
	assert.Equal(t, []experiment.ExcludedTask{
		{TaskID: "t3", Reason: "unaccepted_trial"},
		{TaskID: "t4", Reason: "incompatible_stratum"},
	}, got.ExcludedTasks)
}

func TestCompareExpectedTasks_UnexpectedTrialPreventsExactCompleteness(t *testing.T) {
	t.Parallel()

	got := experiment.CompareExpectedTasks([]experiment.TaskTrial{
		acceptedTrial("expected", "baseline", "AB", canonicalIdentity(), 1000),
		acceptedTrial("expected", "candidate", "AB", canonicalIdentity(), 700),
		acceptedTrial("unexpected", "baseline", "BA", canonicalIdentity(), 1000),
		acceptedTrial("unexpected", "candidate", "BA", canonicalIdentity(), 700),
	}, []string{"expected"})

	assert.Equal(t, 1, got.ExpectedTaskCount)
	assert.Equal(t, 1, got.PairedExpectedTaskCount)
	assert.False(t, got.ExpectedCorpusComplete)
	assert.Equal(t, []string{"unexpected"}, got.UnexpectedTaskIDs)
	assert.Equal(t, []experiment.ExcludedTask{{TaskID: "unexpected", Reason: "unexpected_task"}}, got.ExcludedTasks)
}

func TestCompareCompatibleTasks_DuplicateArmKeepsLegacyExclusionReason(t *testing.T) {
	t.Parallel()

	got := experiment.CompareCompatibleTasks([]experiment.TaskTrial{
		acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 1000),
		acceptedTrial("t1", "baseline", "AB", canonicalIdentity(), 900),
		acceptedTrial("t1", "candidate", "AB", canonicalIdentity(), 700),
	})

	require.Len(t, got.ExcludedTasks, 1)
	assert.Equal(t, experiment.ExcludedTask{TaskID: "t1", Reason: "incompatible_stratum"}, got.ExcludedTasks[0])
}

func acceptedTrial(taskID, arm, order string, identity experiment.ComparisonIdentity, raw int64) experiment.TaskTrial {
	return experiment.TaskTrial{
		TaskID: taskID, Arm: arm, PairOrder: order, Identity: identity,
		Runs: []telemetry.AgentRun{attemptRun(taskID, 1, telemetry.StatusPass, telemetry.StatusPass, raw, arm)},
	}
}

func attemptRun(taskID string, attempt int, status, acceptance string, raw int64, suffix string) telemetry.AgentRun {
	return telemetry.AgentRun{
		AgentName: "executor", TaskID: taskID, Attempt: attempt,
		Status: status, AcceptanceStatus: acceptance,
		Usage: []telemetry.UsageEnvelope{actualUsage(
			"run-"+taskID+"-"+suffix, "call-"+taskID+"-"+suffix, raw,
		)},
	}
}

func actualUsage(runID, callID string, raw int64) telemetry.UsageEnvelope {
	zero := int64(0)
	identity := canonicalIdentity()
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, Source: telemetry.UsageSourceProvider,
		Provider: identity.Provider, Model: identity.Model, Effort: identity.EffortPolicy,
		ProviderVersion: identity.ProviderVersion, ModelVersion: identity.ModelVersion,
		RiskPolicy: identity.RiskPolicy, CacheStratum: identity.CacheStratum, ConfigHash: identity.ConfigHash,
		InputTokensTotal: &raw, OutputTokensTotal: &zero,
	})
}
