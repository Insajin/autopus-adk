package experiment_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/evalregression"
	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestEvaluatePromotion_CriticalRegressionOverridesFortyPercentSavings(t *testing.T) {
	t.Parallel()

	in := promotionFixture(40)
	in.Regressions = []experiment.RegressionEvidence{{
		TaskID: "critical-task", Kind: "objective", Risk: "critical",
		BaselineOutcome: "PASS", CandidateOutcome: "FAIL",
	}}
	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, 1, got.HighCriticalRegressions)
	assert.Equal(t, "PASS", got.Provisional25PctTarget)
	assert.Equal(t, "ROLLBACK", got.RolloutDecision)
	assert.Contains(t, got.ReasonCodes, "high_critical_regression")
}

func TestEvaluatePromotion_TypedRegressionsDeduplicateByTaskAndKind(t *testing.T) {
	t.Parallel()
	in := promotionFixture(40)
	in.Regressions = []experiment.RegressionEvidence{
		{TaskID: "t1", Kind: "objective", Risk: "high", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "t1", Kind: "objective", Risk: "high", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "t1", Kind: "security", Risk: "high", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "t2", Kind: "security", Risk: "critical", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "low", Kind: "security", Risk: "low", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "", Kind: "objective", Risk: "critical", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "bad-kind", Kind: "latency", Risk: "critical", BaselineOutcome: "PASS", CandidateOutcome: "FAIL"},
		{TaskID: "bad-baseline", Kind: "security", Risk: "critical", BaselineOutcome: "FAIL", CandidateOutcome: "FAIL"},
		{TaskID: "no-regression", Kind: "security", Risk: "critical", BaselineOutcome: "PASS", CandidateOutcome: "PASS"},
	}
	got := experiment.EvaluatePromotion(in)
	assert.Equal(t, 3, got.HighCriticalRegressions)
	assert.Equal(t, "ROLLBACK", got.RolloutDecision)
}

func TestEvaluatePromotion_ProvisionalTargetThreshold(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		reduction    float64
		wantTarget   string
		wantDecision string
	}{
		{"twenty six is eligible", 26, "PASS", "ELIGIBLE_NEXT_CANARY"},
		{"twenty four stays exact and blocks", 24, "NOT_MET", "BLOCKED"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := experiment.EvaluatePromotion(promotionFixture(tc.reduction))

			assert.InDelta(t, tc.reduction, got.MeasuredMedianRawReductionPct, 0.001)
			assert.Equal(t, tc.wantTarget, got.Provisional25PctTarget)
			assert.Equal(t, tc.wantDecision, got.RolloutDecision)
		})
	}
}

func TestEvaluatePromotion_IntegrityAndReliabilityFailuresPrecedeSavings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*experiment.PromotionInput)
		reason string
	}{
		{"usage conflict", func(in *experiment.PromotionInput) { in.UsageConflict = true }, telemetry.UsageReasonDuplicateCallConflict},
		{"policy parity", func(in *experiment.PromotionInput) { in.PolicyParityPassed = false }, "policy_parity_failed"},
		{"context integrity", func(in *experiment.PromotionInput) { in.ContextIntegrityPassed = false }, "context_integrity_failed"},
		{"registered reliability", func(in *experiment.PromotionInput) {
			in.ReliabilityDecision = &evalregression.GateDecision{Blocked: true, Reason: "regression_blocked"}
		}, "registered_reliability_regression"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := promotionFixture(60)
			tc.mutate(&in)
			got := experiment.EvaluatePromotion(in)

			assert.Equal(t, "ROLLBACK", got.RolloutDecision)
			assert.Contains(t, got.ReasonCodes, tc.reason)
		})
	}
}

func TestEvaluatePromotion_StrictExpectedCorpusMustBeComplete(t *testing.T) {
	t.Parallel()

	in := promotionFixture(30)
	in.Comparison.ExpectedTaskCount = 2
	in.Comparison.PairedExpectedTaskCount = 1
	in.Comparison.ExpectedCorpusComplete = false

	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, "BLOCKED", got.RolloutDecision)
	assert.Equal(t, []string{"expected_corpus_incomplete"}, got.ReasonCodes)
}

func TestEvaluatePromotion_StrictCompleteExpectedCorpusRemainsEligible(t *testing.T) {
	t.Parallel()

	in := promotionFixture(30)
	in.Comparison.ExpectedTaskCount = 2
	in.Comparison.PairedExpectedTaskCount = 2
	in.Comparison.ExpectedCorpusComplete = true

	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, "ELIGIBLE_NEXT_CANARY", got.RolloutDecision)
	assert.Empty(t, got.ReasonCodes)
}

func TestEvaluatePromotion_IncompleteQualityEvidence_BlocksWithPreciseReason(t *testing.T) {
	t.Parallel()

	in := promotionFixture(40)
	in.CandidateBehaviorActive = false
	in.Quality = &experiment.QualityResult{ExpectedTaskCount: 2, OutcomeRowCount: 1, Complete: false, Consistent: true}

	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, "BLOCKED", got.RolloutDecision)
	assert.Equal(t, []string{"quality_evidence_incomplete"}, got.ReasonCodes)
}

func TestEvaluatePromotion_InconsistentQualityEvidence_ActiveCandidateRollsBack(t *testing.T) {
	t.Parallel()

	in := promotionFixture(40)
	in.Quality = &experiment.QualityResult{ExpectedTaskCount: 2, OutcomeRowCount: 2, Complete: true, Consistent: false}

	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, "ROLLBACK", got.RolloutDecision)
	assert.Equal(t, []string{"quality_evidence_inconsistent"}, got.ReasonCodes)
}

func TestEvaluatePromotion_LowRiskCandidateQualityFailureBlocks(t *testing.T) {
	t.Parallel()

	in := promotionFixture(40)
	in.CandidateBehaviorActive = false
	in.Quality = &experiment.QualityResult{
		ExpectedTaskCount: 1, OutcomeRowCount: 1, Complete: true, Consistent: true,
		CandidateFailureTaskIDs: []string{"task-low"},
	}

	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, "BLOCKED", got.RolloutDecision)
	assert.Equal(t, []string{"candidate_quality_failure"}, got.ReasonCodes)
}

func TestEvaluatePromotion_DerivedQualityRegressions_OverrideCallerRegressions(t *testing.T) {
	t.Parallel()

	in := promotionFixture(40)
	in.Quality = &experiment.QualityResult{
		ExpectedTaskCount: 2, OutcomeRowCount: 2, Complete: true, Consistent: true,
		DerivedRegressions: []experiment.RegressionEvidence{{
			TaskID: "task-high", Kind: "objective", Risk: "high",
			BaselineOutcome: telemetry.StatusPass, CandidateOutcome: telemetry.StatusFail,
		}},
	}
	in.Regressions = []experiment.RegressionEvidence{{
		TaskID: "untrusted", Kind: "security", Risk: "critical",
		BaselineOutcome: telemetry.StatusPass, CandidateOutcome: telemetry.StatusFail,
	}}

	got := experiment.EvaluatePromotion(in)

	assert.Equal(t, 1, got.HighCriticalRegressions)
	assert.Equal(t, "ROLLBACK", got.RolloutDecision)
	assert.Contains(t, got.ReasonCodes, "high_critical_regression")
}

func promotionFixture(reduction float64) experiment.PromotionInput {
	return experiment.PromotionInput{
		Measurement: experiment.MeasurementResult{
			ActualUsageCapturePct: 95, MeasurementGate: "PASS", NeutralityGate: "PASS",
		},
		Comparison: experiment.PairedComparison{
			PairedTaskCount: 2, MedianPairedRawReductionPct: reduction,
		},
		PolicyParityPassed: true, ContextIntegrityPassed: true,
		CurrentStage: "canary", CandidateBehaviorActive: true,
	}
}
