package experiment_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateQualityOutcomes_InexactExpectedRows_ReportIncomplete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		outcomes   []experiment.QualityOutcomeEvidence
		missing    []string
		duplicate  []string
		unexpected []string
	}{
		{name: "missing", outcomes: []experiment.QualityOutcomeEvidence{qualityOutcome("task-a", "high")}, missing: []string{"task-b"}},
		{name: "duplicate", outcomes: []experiment.QualityOutcomeEvidence{
			qualityOutcome("task-a", "high"), qualityOutcome("task-a", "high"), qualityOutcome("task-b", "critical"),
		}, duplicate: []string{"task-a"}},
		{name: "unexpected", outcomes: []experiment.QualityOutcomeEvidence{
			qualityOutcome("task-a", "high"), qualityOutcome("task-b", "critical"), qualityOutcome("task-c", "high"),
		}, unexpected: []string{"task-c"}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := experiment.EvaluateQualityOutcomes([]string{"task-a", "task-b"}, test.outcomes)

			assert.False(t, got.Complete)
			assert.Equal(t, test.missing, got.MissingTaskIDs)
			assert.Equal(t, test.duplicate, got.DuplicateTaskIDs)
			assert.Equal(t, test.unexpected, got.UnexpectedTaskIDs)
		})
	}
}

func TestEvaluateQualityOutcomes_ObservedOutcomes_DeriveRegressions(t *testing.T) {
	t.Parallel()

	objectiveRegression := qualityOutcome("task-high", "high")
	objectiveRegression.CandidateObservedOracleHash = canonicalQualityHash("candidate-mismatch")
	securityRegression := qualityOutcome("task-critical", "critical")
	securityRegression.CandidateSecurityStatus = telemetry.StatusFail

	got := experiment.EvaluateQualityOutcomes(
		[]string{"task-high", "task-critical"},
		[]experiment.QualityOutcomeEvidence{objectiveRegression, securityRegression},
	)

	assert.True(t, got.Complete)
	assert.True(t, got.Consistent)
	assert.Equal(t, 1, got.ObjectivePassCount)
	assert.Equal(t, 1, got.SecurityPassCount)
	require.Len(t, got.DerivedRegressions, 2)
	assert.Equal(t, "objective", got.DerivedRegressions[0].Kind)
	assert.Equal(t, "security", got.DerivedRegressions[1].Kind)
}

func TestEvaluateQualityOutcomes_FailedBaseline_ReportsInconsistent(t *testing.T) {
	t.Parallel()

	outcome := qualityOutcome("task-a", "high")
	outcome.BaselineVerificationExitCode = 1
	outcome.BaselineSecurityStatus = telemetry.StatusFail

	got := experiment.EvaluateQualityOutcomes([]string{"task-a"}, []experiment.QualityOutcomeEvidence{outcome})

	assert.True(t, got.Complete)
	assert.False(t, got.Consistent)
	assert.Equal(t, []string{"task-a"}, got.InconsistentTaskIDs)
	assert.Empty(t, got.DerivedRegressions)
}

func TestEvaluateQualityOutcomes_LowRiskCandidateFailureIsExplicit(t *testing.T) {
	t.Parallel()

	outcome := qualityOutcome("task-low", "low")
	outcome.CandidateSecurityStatus = telemetry.StatusFail

	got := experiment.EvaluateQualityOutcomes([]string{"task-low"}, []experiment.QualityOutcomeEvidence{outcome})

	assert.True(t, got.Complete)
	assert.True(t, got.Consistent)
	assert.Empty(t, got.InconsistentTaskIDs)
	assert.Equal(t, []string{"task-low"}, got.CandidateFailureTaskIDs)
	assert.Empty(t, got.DerivedRegressions)
}

func TestValidateQualityOutcome_NoncanonicalOrUnsafeFields_ReturnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*experiment.QualityOutcomeEvidence)
	}{
		{name: "unsafe task", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.TaskID = "unsafe/task" }},
		{name: "invalid risk", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.RiskTier = "unknown" }},
		{name: "noncanonical task hash", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.TaskHash = "abc" }},
		{name: "uppercase hash", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.ExpectedOracleHash = "sha256:ABCDEF" }},
		{name: "negative exit", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.BaselineVerificationExitCode = -1 }},
		{name: "oversized exit", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.CandidateVerificationExitCode = 256 }},
		{name: "invalid security status", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.CandidateSecurityStatus = "UNKNOWN" }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			outcome := qualityOutcome("task-a", "high")
			test.mutate(&outcome)
			assert.Error(t, experiment.ValidateQualityOutcome(outcome))
		})
	}
}

func qualityOutcome(taskID, risk string) experiment.QualityOutcomeEvidence {
	expected := canonicalQualityHash("oracle-" + taskID)
	return experiment.QualityOutcomeEvidence{
		TaskID: taskID, TaskHash: canonicalQualityHash("task-" + taskID), RiskTier: risk,
		ExpectedOracleHash: expected, BaselineObservedOracleHash: expected, CandidateObservedOracleHash: expected,
		BaselineVerificationExitCode: 0, CandidateVerificationExitCode: 0,
		BaselineSecurityStatus: telemetry.StatusPass, CandidateSecurityStatus: telemetry.StatusPass,
		BaselineSecurityReceiptHash:  canonicalQualityHash("baseline-security-" + taskID),
		CandidateSecurityReceiptHash: canonicalQualityHash("candidate-security-" + taskID),
	}
}

func canonicalQualityHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", digest[:])
}
