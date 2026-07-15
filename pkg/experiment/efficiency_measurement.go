package experiment

import "github.com/insajin/autopus-adk/pkg/telemetry"

const minimumActualCapturePct = 95.0

// EvaluateMeasurement validates A/A instrumentation neutrality and actual
// usage capture before any behavior-changing policy can be promoted.
func EvaluateMeasurement(calls []CallEvidence, neutrality NeutralityEvidence) MeasurementResult {
	usage := make([]telemetry.UsageEnvelope, 0, len(calls))
	identityCompatible := len(calls) > 0 && completeComparisonIdentity(calls[0].Identity)
	for index, call := range calls {
		usage = append(usage, call.Usage)
		if !completeComparisonIdentity(call.Identity) || index > 0 && call.Identity != calls[0].Identity ||
			!usageMatchesComparisonIdentity(call.Usage, call.Identity) {
			identityCompatible = false
		}
	}
	coverage := telemetry.SummarizeUsageCoverage(usage)
	result := MeasurementResult{
		ActualUsageCapturePct: coverage.ActualUsageCapturePct,
		NeutralityGate:        "PASS",
	}

	if coverage.PromotionBlocked {
		result.MeasurementGate = "BLOCKED"
		result.RolloutDecision = "insufficient_measurement"
		result.ReasonCodes = append(result.ReasonCodes, coverage.UnavailableReason)
		return result
	}
	if !identityCompatible {
		result.MeasurementGate = "BLOCKED"
		result.NeutralityGate = "BLOCKED"
		result.RolloutDecision = "insufficient_measurement"
		result.ReasonCodes = []string{"incompatible_measurement_stratum"}
		return result
	}
	result.ReasonCodes = neutralityReasons(neutrality)
	if len(result.ReasonCodes) > 0 {
		result.NeutralityGate = "BLOCKED"
		result.MeasurementGate = "BLOCKED"
		result.RolloutDecision = "instrumentation_not_neutral"
		return result
	}
	if result.ActualUsageCapturePct < minimumActualCapturePct {
		result.MeasurementGate = "BLOCKED"
		result.RolloutDecision = "insufficient_measurement"
		result.ReasonCodes = []string{"actual_usage_below_95_pct"}
		return result
	}
	result.MeasurementGate = "PASS"
	result.RolloutDecision = "sufficient_measurement"
	return result
}

func usageMatchesComparisonIdentity(usage telemetry.UsageEnvelope, identity ComparisonIdentity) bool {
	return usage.Provider != "" && usage.Provider == identity.Provider &&
		usage.ProviderVersion != "" && usage.ProviderVersion == identity.ProviderVersion &&
		usage.Model != "" && usage.Model == identity.Model &&
		usage.ModelVersion != "" && usage.ModelVersion == identity.ModelVersion &&
		usageMatchesEffortPolicy(usage, identity) &&
		usage.RiskPolicy != "" && usage.RiskPolicy == identity.RiskPolicy &&
		usage.CacheStratum != "" && usage.CacheStratum == identity.CacheStratum &&
		usage.ConfigHash != "" && usage.ConfigHash == identity.ConfigHash
}

func usageMatchesEffortPolicy(usage telemetry.UsageEnvelope, identity ComparisonIdentity) bool {
	if identity.EffortPolicy != CodexReviewEffortPolicyV1 {
		return usage.Effort != "" && usage.Effort == identity.EffortPolicy
	}
	if identity.Provider != "codex" || usage.Provider != "codex" {
		return false
	}
	switch usage.Role {
	case "reviewer", "review-consolidator":
		return usage.Effort == "xhigh"
	case "security-auditor":
		return usage.Effort == "max"
	default:
		return false
	}
}

func completeComparisonIdentity(identity ComparisonIdentity) bool {
	return identity.Provider != "" && identity.ProviderVersion != "" &&
		identity.Model != "" && identity.ModelVersion != "" &&
		identity.EffortPolicy != "" && identity.RiskPolicy != "" &&
		identity.CacheStratum != "" && identity.ConfigHash != ""
}

func neutralityReasons(e NeutralityEvidence) []string {
	reasons := make([]string, 0, 3)
	if e.BaselineObjectiveHash == "" || e.BaselineObjectiveHash != e.CandidateObjectiveHash {
		reasons = append(reasons, "objective_changed")
	}
	if e.BaselineCallPolicyHash == "" || e.BaselineCallPolicyHash != e.CandidateCallPolicyHash {
		reasons = append(reasons, "call_policy_changed")
	}
	if e.BaselineAcceptanceHash == "" || e.BaselineAcceptanceHash != e.CandidateAcceptanceHash {
		reasons = append(reasons, "acceptance_changed")
	}
	return reasons
}
