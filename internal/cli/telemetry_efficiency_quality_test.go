package cli

import (
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func TestTelemetryEfficiency_PartialExpectedCorpusIsExplicitAndBlocksPromotion(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	evidence.ExpectedTaskIDs = append(evidence.ExpectedTaskIDs, "task-c")

	result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Comparison.ExpectedTaskCount != 3 || result.Comparison.PairedExpectedTaskCount != 2 || result.Comparison.ExpectedCorpusComplete {
		t.Fatalf("partial corpus was not exposed: %+v", result.Comparison)
	}
	if len(result.Comparison.UnpairedTaskIDs) != 1 || result.Comparison.UnpairedTaskIDs[0] != efficiencyHash("task-c") {
		t.Fatalf("missing expected task disappeared: %+v", result.Comparison)
	}
	if result.Promotion.RolloutDecision != "BLOCKED" || len(result.Promotion.ReasonCodes) != 1 || result.Promotion.ReasonCodes[0] != "expected_corpus_incomplete" {
		t.Fatalf("partial corpus promotion = %+v", result.Promotion)
	}
}

func TestTelemetryEfficiency_MeasurementOnlyEvidenceRemainsValid(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	evidence.Trials = nil
	evidence.ExpectedTaskIDs = nil
	evidence.QualityOutcomes = nil

	result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Measurement.MeasurementGate != "PASS" || result.Comparison.ExpectedTaskCount != 0 || result.Comparison.ExpectedCorpusComplete {
		t.Fatalf("measurement-only result = %+v / %+v", result.Measurement, result.Comparison)
	}
	if result.Promotion.RolloutDecision != "BLOCKED" || len(result.Promotion.ReasonCodes) != 1 || result.Promotion.ReasonCodes[0] != "insufficient_paired_evidence" {
		t.Fatalf("measurement-only promotion = %+v", result.Promotion)
	}
}

func TestTelemetryEfficiency_RejectsInvalidExpectedTaskList(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*telemetryEfficiencyEvidence)
	}{
		{name: "missing", mutate: func(e *telemetryEfficiencyEvidence) { e.ExpectedTaskIDs = nil }},
		{name: "without trials", mutate: func(e *telemetryEfficiencyEvidence) { e.Trials = nil }},
		{name: "duplicate", mutate: func(e *telemetryEfficiencyEvidence) { e.ExpectedTaskIDs = []string{"task-a", "task-a"} }},
		{name: "unsafe", mutate: func(e *telemetryEfficiencyEvidence) { e.ExpectedTaskIDs = []string{"task-a", "unsafe/task"} }},
		{name: "oversized", mutate: func(e *telemetryEfficiencyEvidence) {
			e.ExpectedTaskIDs = make([]string, maxTelemetryEfficiencyExpectedTasks+1)
			for index := range e.ExpectedTaskIDs {
				e.ExpectedTaskIDs[index] = fmt.Sprintf("task-%d", index)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := efficiencyEvidenceFixture(t)
			test.mutate(&evidence)
			if err := validateTelemetryEfficiencyEvidence(evidence); err == nil {
				t.Fatal("invalid expected task list must fail")
			}
		})
	}
}

func TestTelemetryEfficiency_QualityRegressionOverridesSavingsAndRollsBack(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	evidence.CandidateBehaviorActive = true
	evidence.QualityOutcomes[1].CandidateSecurityStatus = telemetry.StatusFail
	result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Promotion.RolloutDecision != "ROLLBACK" || result.RolloutReceipt.Decision != "ROLLBACK" {
		t.Fatalf("promotion/receipt = %+v / %+v", result.Promotion, result.RolloutReceipt)
	}
	if result.RolloutReceipt.ActiveProfile != "full_ultra" || !result.RolloutReceipt.FullDepth {
		t.Fatalf("rollback did not retain full Ultra: %+v", result.RolloutReceipt)
	}
}

func TestTelemetryEfficiency_InexactQualityRows_BlockPromotion(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*telemetryEfficiencyEvidence)
	}{
		{name: "missing", mutate: func(e *telemetryEfficiencyEvidence) { e.QualityOutcomes = e.QualityOutcomes[:1] }},
		{name: "duplicate", mutate: func(e *telemetryEfficiencyEvidence) {
			e.QualityOutcomes = append(e.QualityOutcomes, e.QualityOutcomes[1])
		}},
		{name: "unexpected", mutate: func(e *telemetryEfficiencyEvidence) {
			e.QualityOutcomes = append(e.QualityOutcomes, efficiencyQualityOutcome("task-c", "high"))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := efficiencyEvidenceFixture(t)
			test.mutate(&evidence)
			result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
				"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if result.Quality.Complete || result.Promotion.RolloutDecision != "BLOCKED" ||
				len(result.Promotion.ReasonCodes) != 1 || result.Promotion.ReasonCodes[0] != "quality_evidence_incomplete" {
				t.Fatalf("inexact quality evidence was not blocked: %+v / %+v", result.Quality, result.Promotion)
			}
		})
	}
}

func TestTelemetryEfficiency_InconsistentQualityBaseline_BlocksPromotion(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	evidence.QualityOutcomes[0].BaselineObservedOracleHash = efficiencyHash("wrong-baseline")

	result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Quality.Complete || result.Quality.Consistent || result.Promotion.RolloutDecision != "BLOCKED" ||
		len(result.Promotion.ReasonCodes) != 1 || result.Promotion.ReasonCodes[0] != "quality_evidence_inconsistent" {
		t.Fatalf("inconsistent quality evidence was not blocked: %+v / %+v", result.Quality, result.Promotion)
	}
}

func TestTelemetryEfficiency_LivePairedEvidenceRejectsCallerRegressions(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	evidence.Regressions = []experiment.RegressionEvidence{{
		TaskID: "task-a", Kind: "security", Risk: "critical",
		BaselineOutcome: telemetry.StatusPass, CandidateOutcome: telemetry.StatusFail,
	}}

	if _, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json"); err == nil {
		t.Fatal("paired evidence must reject caller-supplied regressions")
	}
}

func TestTelemetryEfficiency_InvalidQualityFieldsRejectEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*experiment.QualityOutcomeEvidence)
	}{
		{name: "hash", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.TaskHash = "task-hash" }},
		{name: "security status", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.CandidateSecurityStatus = "UNKNOWN" }},
		{name: "exit code", mutate: func(outcome *experiment.QualityOutcomeEvidence) { outcome.CandidateVerificationExitCode = -1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := efficiencyEvidenceFixture(t)
			test.mutate(&evidence.QualityOutcomes[0])
			if _, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
				"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json"); err == nil {
				t.Fatal("invalid quality field must fail strict evidence validation")
			}
		})
	}
}
