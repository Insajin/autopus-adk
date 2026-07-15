package experiment_test

import (
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateMeasurement_ActualCoverageGate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		actual       int
		wantPct      float64
		wantGate     string
		wantDecision string
	}{
		{"nineteen of twenty passes", 19, 95, "PASS", "sufficient_measurement"},
		{"eighteen of twenty blocks", 18, 90, "BLOCKED", "insufficient_measurement"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := experiment.EvaluateMeasurement(measurementCalls(20, tc.actual), neutralHashes())

			assert.InDelta(t, tc.wantPct, got.ActualUsageCapturePct, 0.001)
			assert.Equal(t, tc.wantGate, got.MeasurementGate)
			assert.Equal(t, tc.wantDecision, got.RolloutDecision)
		})
	}
}

func TestEvaluateMeasurement_NeutralityMismatchBlocksAtFullCoverage(t *testing.T) {
	t.Parallel()

	neutrality := neutralHashes()
	neutrality.CandidateCallPolicyHash = "changed-call-policy"
	got := experiment.EvaluateMeasurement(measurementCalls(20, 20), neutrality)

	assert.InDelta(t, 100, got.ActualUsageCapturePct, 0.001)
	assert.Equal(t, "BLOCKED", got.NeutralityGate)
	assert.Equal(t, "BLOCKED", got.MeasurementGate)
	assert.Equal(t, "instrumentation_not_neutral", got.RolloutDecision)
	assert.Contains(t, got.ReasonCodes, "call_policy_changed")
}

func TestEvaluateMeasurement_UsageConflictFailsClosed(t *testing.T) {
	t.Parallel()

	calls := measurementCalls(1, 1)
	conflict := calls[0]
	changed := int64(999)
	zero := int64(0)
	conflict.Usage = telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: conflict.Usage.RunID, CallID: conflict.Usage.CallID,
		Source:           telemetry.UsageSourceProvider,
		InputTokensTotal: &changed, OutputTokensTotal: &zero,
	})
	calls = append(calls, conflict)

	got := experiment.EvaluateMeasurement(calls, neutralHashes())

	assert.Equal(t, "BLOCKED", got.MeasurementGate)
	assert.Contains(t, got.ReasonCodes, telemetry.UsageReasonDuplicateCallConflict)
}

func TestEvaluateMeasurement_EnvelopeProvenanceMismatchBlocks(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*telemetry.UsageEnvelope)
	}{
		{name: "provider", mutate: func(v *telemetry.UsageEnvelope) { v.Provider = "spoof" }},
		{name: "model", mutate: func(v *telemetry.UsageEnvelope) { v.Model = "spoof" }},
		{name: "effort missing", mutate: func(v *telemetry.UsageEnvelope) { v.Effort = "" }},
		{name: "provider version", mutate: func(v *telemetry.UsageEnvelope) { v.ProviderVersion = "spoof" }},
		{name: "model version missing", mutate: func(v *telemetry.UsageEnvelope) { v.ModelVersion = "" }},
		{name: "risk policy", mutate: func(v *telemetry.UsageEnvelope) { v.RiskPolicy = "spoof" }},
		{name: "cache stratum missing", mutate: func(v *telemetry.UsageEnvelope) { v.CacheStratum = "" }},
		{name: "config hash", mutate: func(v *telemetry.UsageEnvelope) { v.ConfigHash = "spoof" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := measurementCalls(1, 1)
			test.mutate(&calls[0].Usage)
			got := experiment.EvaluateMeasurement(calls, neutralHashes())
			assert.Equal(t, "BLOCKED", got.MeasurementGate)
			assert.Contains(t, got.ReasonCodes, "incompatible_measurement_stratum")
		})
	}
}

func measurementCalls(total, actual int) []experiment.CallEvidence {
	identity := canonicalIdentity()
	out := make([]experiment.CallEvidence, 0, total)
	for i := 0; i < total; i++ {
		runID := fmt.Sprintf("run-%02d", i)
		callID := fmt.Sprintf("call-%02d", i)
		var usage telemetry.UsageEnvelope
		if i < actual {
			usage = actualUsage(runID, callID, 100)
		} else {
			usage = telemetry.NormalizeUsage(telemetry.UsageInput{
				RunID: runID, CallID: callID, Source: telemetry.UsageSourceProvider,
				Provider: identity.Provider, Model: identity.Model, Effort: identity.EffortPolicy,
				ProviderVersion: identity.ProviderVersion, ModelVersion: identity.ModelVersion,
				RiskPolicy: identity.RiskPolicy, CacheStratum: identity.CacheStratum, ConfigHash: identity.ConfigHash,
			})
		}
		out = append(out, experiment.CallEvidence{Usage: usage, Identity: identity})
	}
	return out
}

func neutralHashes() experiment.NeutralityEvidence {
	return experiment.NeutralityEvidence{
		BaselineObjectiveHash:   "objective-hash",
		CandidateObjectiveHash:  "objective-hash",
		BaselineCallPolicyHash:  "call-policy-hash",
		CandidateCallPolicyHash: "call-policy-hash",
		BaselineAcceptanceHash:  "acceptance-hash",
		CandidateAcceptanceHash: "acceptance-hash",
	}
}

func canonicalIdentity() experiment.ComparisonIdentity {
	return experiment.ComparisonIdentity{
		Provider: "claude", ProviderVersion: "2.1.154",
		Model: "claude-opus-4-8", ModelVersion: "2026-07-01",
		EffortPolicy: "max", RiskPolicy: "risk-v1",
		CacheStratum: "cold", ConfigHash: "config-hash",
	}
}
