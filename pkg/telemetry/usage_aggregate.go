package telemetry

// UsageAggregate summarizes immutable per-call receipts.
type UsageAggregate struct {
	UniqueModelCallCount int      `json:"unique_model_call_count"`
	RawTotalTokens       *int64   `json:"raw_total_tokens"`
	ActualCostUSD        *float64 `json:"actual_cost_usd"`
	EstimatedTotalTokens *int64   `json:"estimated_total_tokens"`
	EstimatedCostUSD     *float64 `json:"estimated_cost_usd"`
	UsageStatus          string   `json:"usage_status"`
	UnavailableReason    string   `json:"unavailable_reason,omitempty"`
	PromotionBlocked     bool     `json:"promotion_blocked"`
}

type callIdentity struct{ runID, callID string }

// AggregateUsage deterministically counts each run/call identity once. A
// conflicting duplicate invalidates the aggregate and blocks policy promotion.
// @AX:ANCHOR: [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001 — canonical model-call deduplication and conflict boundary.
// @AX:REASON: Worker, orchestra, CLI persistence, coverage, and direct-agent paths all depend on identical run/call aggregation semantics.
func AggregateUsage(inputs []UsageEnvelope) UsageAggregate {
	result := UsageAggregate{UsageStatus: UsageStatusUnavailable}
	seen := make(map[callIdentity]UsageEnvelope, len(inputs))
	unique := make([]UsageEnvelope, 0, len(inputs))
	for _, input := range inputs {
		if input.RunID == "" || input.CallID == "" {
			result.PromotionBlocked = true
			result.UnavailableReason = UsageReasonIdentityMissing
			continue
		}
		identity := callIdentity{runID: input.RunID, callID: input.CallID}
		if prior, exists := seen[identity]; exists {
			if !sameUsageAggregationSemantics(prior, input) {
				result.PromotionBlocked = true
				if result.UnavailableReason != UsageReasonIdentityMissing {
					result.UnavailableReason = UsageReasonDuplicateCallConflict
				}
			}
			continue
		}
		seen[identity] = input
		unique = append(unique, input)
	}
	result.UniqueModelCallCount = len(seen)
	if result.PromotionBlocked {
		return result
	}

	var rawTotal, estimatedTotal int64
	var actualCost, estimatedCost float64
	hasEstimated, hasActualCost, hasEstimatedCost := false, false, false
	allActual := len(unique) > 0
	allCostOnly := len(unique) > 0
	allEstimated := len(unique) > 0
	for _, envelope := range unique {
		if envelope.UsageStatus != UsageStatusActual || envelope.RawTotalTokens == nil {
			allActual = false
		}
		if envelope.UsageStatus != UsageStatusCostOnly {
			allCostOnly = false
		}
		if envelope.UsageStatus != UsageStatusEstimated {
			allEstimated = false
		}
		if envelope.RawTotalTokens != nil {
			rawTotal += *envelope.RawTotalTokens
		}
		if envelope.EstimatedTotalTokens != nil {
			estimatedTotal += *envelope.EstimatedTotalTokens
			hasEstimated = true
		}
		if envelope.ActualCostUSD != nil {
			actualCost += *envelope.ActualCostUSD
			hasActualCost = true
		}
		if envelope.EstimatedCostUSD != nil {
			estimatedCost += *envelope.EstimatedCostUSD
			hasEstimatedCost = true
		}
	}
	if hasActualCost {
		result.ActualCostUSD = float64Pointer(actualCost)
	}
	if hasEstimated {
		result.EstimatedTotalTokens = int64Pointer(estimatedTotal)
	}
	if hasEstimatedCost {
		result.EstimatedCostUSD = float64Pointer(estimatedCost)
	}
	if allActual {
		result.RawTotalTokens = int64Pointer(rawTotal)
		result.UsageStatus = UsageStatusActual
		return result
	}
	if allCostOnly {
		result.UsageStatus = UsageStatusCostOnly
		return result
	}
	if allEstimated {
		result.UsageStatus = UsageStatusEstimated
		return result
	}
	result.UnavailableReason = firstUnavailableReason(unique)
	return result
}

func firstUnavailableReason(envelopes []UsageEnvelope) string {
	for _, envelope := range envelopes {
		if envelope.UnavailableReason != "" {
			return envelope.UnavailableReason
		}
	}
	return UsageReasonProviderAbsent
}

func float64Pointer(value float64) *float64 { return &value }

func sameUsageAggregationSemantics(left, right UsageEnvelope) bool {
	return left.Version == right.Version &&
		left.Provider == right.Provider && left.Model == right.Model && left.Effort == right.Effort &&
		left.ProviderVersion == right.ProviderVersion && left.ModelVersion == right.ModelVersion &&
		left.RiskPolicy == right.RiskPolicy && left.CacheStratum == right.CacheStratum &&
		left.ConfigHash == right.ConfigHash &&
		left.UsageStatus == right.UsageStatus && left.UsageSource == right.UsageSource &&
		left.SourceSchema == right.SourceSchema && left.UnavailableReason == right.UnavailableReason &&
		equalInt64(left.InputTokensTotal, right.InputTokensTotal) &&
		equalInt64(left.UncachedInputTokens, right.UncachedInputTokens) &&
		equalInt64(left.CachedInputTokens, right.CachedInputTokens) &&
		equalInt64(left.CacheCreationInputTokens, right.CacheCreationInputTokens) &&
		equalInt64(left.CacheReadInputTokens, right.CacheReadInputTokens) &&
		equalInt64(left.OutputTokensTotal, right.OutputTokensTotal) &&
		equalInt64(left.ReasoningTokens, right.ReasoningTokens) &&
		left.ReasoningRelation == right.ReasoningRelation &&
		equalInt64(left.ToolTokens, right.ToolTokens) &&
		left.ToolRelation == right.ToolRelation &&
		equalInt64(left.RawTotalTokens, right.RawTotalTokens) &&
		equalFloat64(left.ActualCostUSD, right.ActualCostUSD) &&
		equalInt64(left.EstimatedTotalTokens, right.EstimatedTotalTokens) &&
		equalFloat64(left.EstimatedCostUSD, right.EstimatedCostUSD) &&
		left.PricingVersion == right.PricingVersion
}

func sameActualUsage(left, right UsageEnvelope) bool {
	return sameUsageAggregationSemantics(left, right)
}

func equalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func equalFloat64(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
