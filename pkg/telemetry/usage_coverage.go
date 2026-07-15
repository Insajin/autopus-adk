package telemetry

// UsageCoverage summarizes actual-complete capture across unique model calls.
// Conflicting duplicate identities fail closed and block promotion.
type UsageCoverage struct {
	EligibleCallCount       int     `json:"eligible_call_count"`
	ActualCompleteCallCount int     `json:"actual_complete_call_count"`
	ActualUsageCapturePct   float64 `json:"actual_usage_capture_pct"`
	UnavailableReason       string  `json:"unavailable_reason,omitempty"`
	PromotionBlocked        bool    `json:"promotion_blocked"`
}

// SummarizeUsageCoverage applies the canonical usage identity and conflict
// semantics before calculating actual-complete capture percentage.
func SummarizeUsageCoverage(inputs []UsageEnvelope) UsageCoverage {
	aggregate := AggregateUsage(inputs)
	result := UsageCoverage{
		EligibleCallCount: aggregate.UniqueModelCallCount,
		PromotionBlocked:  aggregate.PromotionBlocked,
		UnavailableReason: aggregate.UnavailableReason,
	}
	if result.PromotionBlocked {
		return result
	}

	unique, conflict := uniqueUsage(inputs)
	if conflict != "" {
		result.PromotionBlocked = true
		result.UnavailableReason = conflict
		return result
	}
	for _, envelope := range unique {
		if envelope.UsageStatus == UsageStatusActual && envelope.RawTotalTokens != nil {
			result.ActualCompleteCallCount++
		}
	}
	if result.EligibleCallCount > 0 {
		result.ActualUsageCapturePct = float64(result.ActualCompleteCallCount) /
			float64(result.EligibleCallCount) * 100
	}
	return result
}
