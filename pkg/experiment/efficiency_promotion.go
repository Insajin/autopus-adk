package experiment

import "github.com/insajin/autopus-adk/pkg/telemetry"

// EvaluatePromotion applies quality, integrity, and reliability barriers before
// considering the provisional raw-token reduction target.
func EvaluatePromotion(input PromotionInput) PromotionResult {
	regressions := input.Regressions
	if input.Quality != nil {
		regressions = input.Quality.DerivedRegressions
	}
	result := PromotionResult{
		HighCriticalRegressions:       countHighCriticalRegressions(regressions),
		MeasuredMedianRawReductionPct: input.Comparison.MedianPairedRawReductionPct,
		Provisional25PctTarget:        "NOT_MET",
	}
	if result.MeasuredMedianRawReductionPct >= 25 {
		result.Provisional25PctTarget = "PASS"
	}

	if result.HighCriticalRegressions > 0 {
		result.ReasonCodes = append(result.ReasonCodes, "high_critical_regression")
	}
	if input.UsageConflict {
		result.ReasonCodes = append(result.ReasonCodes, telemetry.UsageReasonDuplicateCallConflict)
	}
	if !input.PolicyParityPassed {
		result.ReasonCodes = append(result.ReasonCodes, "policy_parity_failed")
	}
	if !input.ContextIntegrityPassed {
		result.ReasonCodes = append(result.ReasonCodes, "context_integrity_failed")
	}
	if input.ReliabilityDecision != nil && input.ReliabilityDecision.Blocked {
		result.ReasonCodes = append(result.ReasonCodes, "registered_reliability_regression")
	}
	if len(result.ReasonCodes) > 0 {
		result.RolloutDecision = "BLOCKED"
		if input.CandidateBehaviorActive {
			result.RolloutDecision = "ROLLBACK"
		}
		return result
	}
	if input.Measurement.NeutralityGate != "PASS" || input.Measurement.MeasurementGate != "PASS" {
		result.RolloutDecision = "BLOCKED"
		result.ReasonCodes = []string{"insufficient_measurement"}
		return result
	}
	if input.Comparison.ExpectedTaskCount > 0 && !input.Comparison.ExpectedCorpusComplete {
		result.RolloutDecision = "BLOCKED"
		result.ReasonCodes = []string{"expected_corpus_incomplete"}
		return result
	}
	if input.Quality != nil && !input.Quality.Complete {
		result.RolloutDecision = qualityFailureDecision(input.CandidateBehaviorActive)
		result.ReasonCodes = []string{"quality_evidence_incomplete"}
		return result
	}
	if input.Quality != nil && !input.Quality.Consistent {
		result.RolloutDecision = qualityFailureDecision(input.CandidateBehaviorActive)
		result.ReasonCodes = []string{"quality_evidence_inconsistent"}
		return result
	}
	if input.Quality != nil && len(input.Quality.CandidateFailureTaskIDs) > 0 {
		result.RolloutDecision = qualityFailureDecision(input.CandidateBehaviorActive)
		result.ReasonCodes = []string{"candidate_quality_failure"}
		return result
	}
	if input.Comparison.PairedTaskCount == 0 {
		result.RolloutDecision = "BLOCKED"
		result.ReasonCodes = []string{"insufficient_paired_evidence"}
		return result
	}
	if result.Provisional25PctTarget != "PASS" {
		result.RolloutDecision = "BLOCKED"
		result.ReasonCodes = []string{"provisional_target_not_met"}
		return result
	}
	result.RolloutDecision = "ELIGIBLE_NEXT_CANARY"
	return result
}

func qualityFailureDecision(candidateBehaviorActive bool) string {
	if candidateBehaviorActive {
		return "ROLLBACK"
	}
	return "BLOCKED"
}

func countHighCriticalRegressions(evidence []RegressionEvidence) int {
	type regressionKey struct{ taskID, kind string }
	regressions := make(map[regressionKey]struct{})
	for _, item := range evidence {
		if item.TaskID == "" || item.Risk != "high" && item.Risk != "critical" ||
			item.Kind != "objective" && item.Kind != "security" ||
			item.BaselineOutcome != "PASS" || item.CandidateOutcome == "PASS" {
			continue
		}
		regressions[regressionKey{taskID: item.TaskID, kind: item.Kind}] = struct{}{}
	}
	return len(regressions)
}
