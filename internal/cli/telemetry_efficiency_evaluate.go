package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/evalregression"
	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func validateTelemetryEfficiencyEvidence(e telemetryEfficiencyEvidence) error {
	if e.Version != telemetryEfficiencyEvidenceVersion || len(e.Calls) == 0 || len(e.Calls) > maxTelemetryEfficiencyCalls ||
		len(e.Trials) > maxTelemetryEfficiencyTrials || len(e.QualityOutcomes) > maxTelemetryEfficiencyExpectedTasks ||
		len(e.Regressions) > maxTelemetryEfficiencyRegressions {
		return errors.New("invalid evidence bounds")
	}
	pairedEvidence := len(e.Trials) > 0
	if !validTelemetryEfficiencyExpectedTasks(e.ExpectedTaskIDs, pairedEvidence) ||
		!pairedEvidence && len(e.QualityOutcomes) > 0 || pairedEvidence && len(e.Regressions) > 0 {
		return errors.New("invalid expected task list")
	}
	for _, call := range e.Calls {
		if telemetry.ValidateUsageEnvelope(call.Usage) != nil {
			return errors.New("invalid call usage")
		}
	}
	for _, trial := range e.Trials {
		if !safeTelemetryEfficiencyID(trial.TaskID) || len(trial.Runs) > maxTelemetryEfficiencyRunsPerTrial ||
			(trial.Arm != "baseline" && trial.Arm != "candidate") || (trial.PairOrder != "AB" && trial.PairOrder != "BA") {
			return errors.New("invalid trial")
		}
		for _, run := range trial.Runs {
			if run.TaskID != trial.TaskID || !validTelemetryEfficiencyStatus(run.Status, false) ||
				!validTelemetryEfficiencyStatus(run.AcceptanceStatus, true) {
				return errors.New("invalid trial run")
			}
			for _, usage := range run.Usage {
				if usage.TaskID != "" && usage.TaskID != trial.TaskID || telemetry.ValidateUsageEnvelope(usage) != nil {
					return errors.New("invalid trial usage")
				}
			}
		}
	}
	for _, outcome := range e.QualityOutcomes {
		if experiment.ValidateQualityOutcome(outcome) != nil {
			return errors.New("invalid quality outcome")
		}
	}
	if !validTelemetryEfficiencyRegressions(e.Regressions) || !validTelemetryEfficiencyPolicy(e) {
		return errors.New("invalid policy evidence")
	}
	return nil
}

func validTelemetryEfficiencyExpectedTasks(taskIDs []string, required bool) bool {
	if len(taskIDs) > maxTelemetryEfficiencyExpectedTasks || required != (len(taskIDs) > 0) {
		return false
	}
	seen := make(map[string]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		if !safeTelemetryEfficiencyID(taskID) {
			return false
		}
		if _, duplicate := seen[taskID]; duplicate {
			return false
		}
		seen[taskID] = struct{}{}
	}
	return true
}

func validTelemetryEfficiencyRegressions(items []experiment.RegressionEvidence) bool {
	for _, item := range items {
		if !safeTelemetryEfficiencyID(item.TaskID) || item.Kind != "objective" && item.Kind != "security" ||
			!oneOf(item.Risk, "low", "medium", "high", "critical") ||
			!validTelemetryEfficiencyStatus(item.BaselineOutcome, false) || !validTelemetryEfficiencyStatus(item.CandidateOutcome, false) {
			return false
		}
	}
	return true
}

func validTelemetryEfficiencyPolicy(e telemetryEfficiencyEvidence) bool {
	if !oneOf(e.CurrentStage, "", "shadow", "canary") || !oneOf(e.Rollout.ReceiptKind, "", "shadow", "canary") ||
		!oneOf(e.Rollout.RiskTier, "", "low", "medium", "high", "critical", "sensitive", "unknown") {
		return false
	}
	for _, value := range []string{e.Rollout.ExperimentID, e.Rollout.TaskCorpusHash, e.Rollout.PolicyHash, e.Rollout.ConfigHash} {
		if value == "" || len(value) > maxTelemetryEfficiencyStringBytes {
			return false
		}
	}
	if len(e.Trials) > 0 {
		for _, value := range []string{e.Rollout.TaskCorpusHash, e.Rollout.PolicyHash, e.Rollout.ConfigHash} {
			if !canonicalReceiptHash(value) {
				return false
			}
		}
		if e.Rollout.AuditTaskHash != "" && !validAuditQualityRow(
			e.QualityOutcomes, e.Rollout.AuditTaskHash, e.Rollout.RiskTier,
		) {
			return false
		}
	}
	if e.Rollout.AuditTaskHash == "" {
		if e.Rollout.AuditRatePercent != 0 {
			return false
		}
	} else if _, err := experiment.SelectFullDepthAudit(
		e.Rollout.AuditTaskHash, e.Rollout.PolicyHash, e.Rollout.AuditRatePercent,
	); err != nil {
		return false
	}
	if e.Reliability != nil && (e.Reliability.ExitCode < 0 || e.Reliability.ExitCode > 1 ||
		len(e.Reliability.Reason) > maxTelemetryEfficiencyStringBytes || len(e.Reliability.AttributedVersion) > maxTelemetryEfficiencyStringBytes) {
		return false
	}
	return true
}

func validAuditQualityRow(outcomes []experiment.QualityOutcomeEvidence, taskHash, rolloutRisk string) bool {
	matches := 0
	for _, outcome := range outcomes {
		if outcome.TaskHash != taskHash {
			continue
		}
		matches++
		if !oneOf(outcome.RiskTier, "low", "medium") || outcome.RiskTier != rolloutRisk {
			return false
		}
	}
	return matches == 1
}

func validTelemetryEfficiencyStatus(value string, optional bool) bool {
	return optional && value == "" || value == telemetry.StatusPass || value == telemetry.StatusFail
}

func safeTelemetryEfficiencyID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-_.:", char)) {
			return false
		}
	}
	return true
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func evaluateTelemetryEfficiency(e telemetryEfficiencyEvidence, now time.Time) telemetryEfficiencyResult {
	measurement := experiment.EvaluateMeasurement(e.Calls, e.Neutrality)
	comparison := experiment.CompareCompatibleTasks(e.Trials)
	strictExpectedCorpus := len(e.Trials) > 0
	if strictExpectedCorpus {
		comparison = experiment.CompareExpectedTasks(e.Trials, e.ExpectedTaskIDs)
	}
	var quality *experiment.QualityResult
	if strictExpectedCorpus {
		result := experiment.EvaluateQualityOutcomes(e.ExpectedTaskIDs, e.QualityOutcomes)
		quality = &result
	}
	var reliability *evalregression.GateDecision
	if e.Reliability != nil {
		reliability = &evalregression.GateDecision{
			Blocked: e.Reliability.Blocked, ExitCode: e.Reliability.ExitCode,
			Reason: e.Reliability.Reason, AttributedVersion: e.Reliability.AttributedVersion,
		}
	}
	promotion := experiment.EvaluatePromotion(experiment.PromotionInput{
		Measurement: measurement, Comparison: comparison, Quality: quality, Regressions: e.Regressions,
		UsageConflict: e.UsageConflict, PolicyParityPassed: e.PolicyParityPassed,
		ContextIntegrityPassed: e.ContextIntegrityPassed, ReliabilityDecision: reliability,
		CurrentStage: e.CurrentStage, CandidateBehaviorActive: e.CandidateBehaviorActive,
	})
	receiptKind, audit := resolvedTelemetryEfficiencyAudit(e.Rollout)
	receipt := experiment.BuildRolloutReceipt(experiment.RolloutReceiptInput{
		ExperimentID: e.Rollout.ExperimentID, TaskCorpusHash: e.Rollout.TaskCorpusHash,
		PolicyHash: e.Rollout.PolicyHash, ConfigHash: e.Rollout.ConfigHash,
		ReceiptKind: receiptKind, RiskTier: e.Rollout.RiskTier,
		Sensitive: e.Rollout.Sensitive, FullDepth: e.Rollout.FullDepth,
		AuditSelection: audit, Promotion: promotion,
	}, now)
	result := telemetryEfficiencyResult{Version: telemetryEfficiencyResultVersion, Measurement: measurement,
		Comparison: digestComparisonTaskIdentities(comparison), Promotion: promotion, RolloutReceipt: receipt}
	if quality != nil {
		result.Quality = digestQualityTaskIdentities(*quality)
	}
	return result
}

func resolvedTelemetryEfficiencyAudit(rollout telemetryEfficiencyRolloutEvidence) (string, experiment.AuditSelection) {
	kind := rollout.ReceiptKind
	if rollout.AuditTaskHash == "" {
		if kind == "canary" && (rollout.RiskTier == "low" || rollout.RiskTier == "medium") {
			kind = "shadow"
		}
		return kind, experiment.AuditSelection{}
	}
	audit, err := experiment.SelectFullDepthAudit(rollout.AuditTaskHash, rollout.PolicyHash, rollout.AuditRatePercent)
	if err != nil {
		return "shadow", experiment.AuditSelection{}
	}
	if audit.Selected {
		kind = "audit_sample"
	}
	return kind, audit
}

func digestComparisonTaskIdentities(input experiment.PairedComparison) experiment.PairedComparison {
	result := input
	result.PairedTaskIDs = digestTaskIDs(input.PairedTaskIDs)
	result.UnpairedTaskIDs = digestTaskIDs(input.UnpairedTaskIDs)
	result.ExpectedTaskIDs = digestTaskIDs(input.ExpectedTaskIDs)
	result.UnexpectedTaskIDs = digestTaskIDs(input.UnexpectedTaskIDs)
	result.ABTaskIDs = digestTaskIDs(input.ABTaskIDs)
	result.BATaskIDs = digestTaskIDs(input.BATaskIDs)
	result.ExcludedTasks = make([]experiment.ExcludedTask, len(input.ExcludedTasks))
	for index, task := range input.ExcludedTasks {
		result.ExcludedTasks[index] = experiment.ExcludedTask{TaskID: digestTaskID(task.TaskID), Reason: task.Reason}
	}
	return result
}

func digestTaskIDs(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = digestTaskID(value)
	}
	return result
}

func digestTaskID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func digestQualityTaskIdentities(input experiment.QualityResult) experiment.QualityResult {
	result := input
	result.MissingTaskIDs = digestTaskIDs(input.MissingTaskIDs)
	result.DuplicateTaskIDs = digestTaskIDs(input.DuplicateTaskIDs)
	result.UnexpectedTaskIDs = digestTaskIDs(input.UnexpectedTaskIDs)
	result.InconsistentTaskIDs = digestTaskIDs(input.InconsistentTaskIDs)
	result.CandidateFailureTaskIDs = digestTaskIDs(input.CandidateFailureTaskIDs)
	result.DerivedRegressions = make([]experiment.RegressionEvidence, len(input.DerivedRegressions))
	for index, regression := range input.DerivedRegressions {
		regression.TaskID = digestTaskID(regression.TaskID)
		result.DerivedRegressions[index] = regression
	}
	return result
}
