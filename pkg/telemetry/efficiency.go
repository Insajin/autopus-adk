package telemetry

import "math"

const (
	EfficiencyReasonZeroAcceptedTasks  = "zero_accepted_tasks"
	EfficiencyReasonActualIncomplete   = "actual_usage_incomplete"
	EfficiencyReasonAcceptanceConflict = "conflicting_final_acceptance"
)

// EfficiencySummary keeps objective acceptance, raw usage, and billable spend
// separate. RawTokens includes all actual-complete attempts, including failures.
type EfficiencySummary struct {
	ActualCoverage                float64  `json:"actual_coverage"`
	RawTokens                     int64    `json:"raw_tokens"`
	RawTotalTokens                int64    `json:"raw_total_tokens"`
	ActualCostUSD                 *float64 `json:"actual_cost_usd"`
	BillableActualCostUSD         *float64 `json:"billable_actual_cost_usd"`
	EstimatedTokens               int64    `json:"estimated_tokens"`
	EstimatedCostUSD              *float64 `json:"estimated_cost_usd"`
	BillableEstimatedCostUSD      *float64 `json:"billable_estimated_cost_usd"`
	UniqueModelCallCount          int      `json:"unique_model_call_count"`
	ToolCallCount                 int      `json:"tool_call_count"`
	FailedSpendRawTokens          int64    `json:"failed_spend_raw_tokens"`
	AcceptedTasks                 int      `json:"accepted_tasks"`
	RawTotalTokensPerAcceptedTask *float64 `json:"raw_total_tokens_per_accepted_task"`
	UnavailableReason             string   `json:"unavailable_reason,omitempty"`
	PromotionBlocked              bool     `json:"promotion_blocked"`
}

// SpendComparison demonstrates raw-token and billable-cost changes separately.
type SpendComparison struct {
	ColdRawTotalTokens     int64    `json:"cold_raw_total_tokens"`
	WarmRawTotalTokens     int64    `json:"warm_raw_total_tokens"`
	RawTokenReductionPct   float64  `json:"raw_token_reduction_pct"`
	ColdActualCostUSD      *float64 `json:"cold_actual_cost_usd"`
	WarmActualCostUSD      *float64 `json:"warm_actual_cost_usd"`
	ActualCostReductionPct *float64 `json:"actual_cost_reduction_pct"`
	UnavailableReason      string   `json:"unavailable_reason,omitempty"`
}

// SummarizeEfficiency evaluates every unique model call and the final objective
// acceptance status of each distinct task.
// @AX:ANCHOR: [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001 — accepted-task efficiency public contract shared by reports, CLI, and paired experiments.
// @AX:REASON: Six production call sites rely on all-attempt spend and distinct final-acceptance denominator semantics.
func SummarizeEfficiency(runs []AgentRun) EfficiencySummary {
	result := EfficiencySummary{}
	taskFinal := make(map[string]finalAcceptance)
	allUsage := make([]UsageEnvelope, 0)
	legacyEstimates := int64(0)

	for i, run := range runs {
		taskID := run.TaskID
		if taskID == "" {
			taskID = taskIdentityFromUsage(run.Usage)
		}
		if taskID == "" {
			taskID = run.AgentName + "#" + itoa(i)
		}
		if run.AcceptanceStatus != "" {
			candidate := finalAcceptance{attempt: run.Attempt, accepted: run.Status == StatusPass && run.AcceptanceStatus == StatusPass}
			prior, exists := taskFinal[taskID]
			switch {
			case !exists || candidate.attempt > prior.attempt:
				taskFinal[taskID] = candidate
			case candidate.attempt == prior.attempt && candidate.accepted != prior.accepted:
				prior.conflict = true
				taskFinal[taskID] = prior
			}
		}
		result.ToolCallCount += run.ToolCalls
		legacyEstimates += int64(run.EstimatedTokens)
		for _, envelope := range run.Usage {
			if envelope.TaskID == "" {
				envelope.TaskID = taskID
			}
			allUsage = append(allUsage, envelope)
		}
	}

	acceptanceConflict := false
	for _, final := range taskFinal {
		if final.conflict {
			acceptanceConflict = true
		}
		if final.accepted && !final.conflict {
			result.AcceptedTasks++
		}
	}

	unique, conflictReason := uniqueUsage(allUsage)
	result.UniqueModelCallCount = len(unique)
	actualCalls := 0
	var actualCost, estimatedCost float64
	hasActualCost, hasEstimatedCost := false, false
	for _, envelope := range unique {
		if envelope.UsageStatus == UsageStatusActual && envelope.RawTotalTokens != nil {
			actualCalls++
			result.RawTokens += *envelope.RawTotalTokens
			if final := taskFinal[envelope.TaskID]; !final.accepted || final.conflict {
				result.FailedSpendRawTokens += *envelope.RawTotalTokens
			}
		}
		if envelope.ActualCostUSD != nil {
			actualCost += *envelope.ActualCostUSD
			hasActualCost = true
		}
		if envelope.EstimatedTotalTokens != nil {
			result.EstimatedTokens += *envelope.EstimatedTotalTokens
		}
		if envelope.EstimatedCostUSD != nil {
			estimatedCost += *envelope.EstimatedCostUSD
			hasEstimatedCost = true
		}
	}
	result.EstimatedTokens += legacyEstimates
	result.RawTotalTokens = result.RawTokens
	if hasActualCost {
		result.ActualCostUSD = float64Pointer(actualCost)
		result.BillableActualCostUSD = float64Pointer(actualCost)
	}
	if hasEstimatedCost {
		result.EstimatedCostUSD = float64Pointer(estimatedCost)
		result.BillableEstimatedCostUSD = float64Pointer(estimatedCost)
	}
	if result.UniqueModelCallCount > 0 {
		result.ActualCoverage = float64(actualCalls) / float64(result.UniqueModelCallCount)
	}

	switch {
	case acceptanceConflict:
		result.UnavailableReason = EfficiencyReasonAcceptanceConflict
		result.PromotionBlocked = true
	case conflictReason != "":
		result.UnavailableReason = conflictReason
		result.PromotionBlocked = true
	case result.AcceptedTasks == 0:
		result.UnavailableReason = EfficiencyReasonZeroAcceptedTasks
	case actualCalls != result.UniqueModelCallCount || result.UniqueModelCallCount == 0:
		result.UnavailableReason = EfficiencyReasonActualIncomplete
	default:
		value := float64(result.RawTokens) / float64(result.AcceptedTasks)
		result.RawTotalTokensPerAcceptedTask = &value
	}
	return result
}

type finalAcceptance struct {
	attempt  int
	accepted bool
	conflict bool
}

// CompareUsageSpend compares inclusive raw totals independently of cache-priced
// billable cost. Cached tokens are never subtracted from inclusive input.
func CompareUsageSpend(cold, warm []UsageEnvelope) SpendComparison {
	left := usageSpend(cold)
	right := usageSpend(warm)
	result := SpendComparison{
		ColdRawTotalTokens: left.raw, WarmRawTotalTokens: right.raw,
		ColdActualCostUSD: left.cost, WarmActualCostUSD: right.cost,
	}
	if left.complete && right.complete && left.raw != 0 {
		result.RawTokenReductionPct = percentReduction(float64(left.raw), float64(right.raw))
	} else {
		result.UnavailableReason = EfficiencyReasonActualIncomplete
	}
	if left.cost != nil && right.cost != nil && *left.cost != 0 {
		value := percentReduction(*left.cost, *right.cost)
		result.ActualCostReductionPct = &value
	}
	return result
}

type spend struct {
	raw      int64
	cost     *float64
	complete bool
}

func usageSpend(envelopes []UsageEnvelope) spend {
	unique, conflict := uniqueUsage(envelopes)
	result := spend{complete: conflict == "" && len(unique) > 0}
	var cost float64
	hasCost := false
	for _, envelope := range unique {
		if envelope.UsageStatus != UsageStatusActual || envelope.RawTotalTokens == nil {
			result.complete = false
		} else {
			result.raw += *envelope.RawTotalTokens
		}
		if envelope.ActualCostUSD != nil {
			cost += *envelope.ActualCostUSD
			hasCost = true
		}
	}
	if hasCost {
		result.cost = float64Pointer(cost)
	}
	return result
}

func uniqueUsage(inputs []UsageEnvelope) ([]UsageEnvelope, string) {
	seen := make(map[callIdentity]UsageEnvelope, len(inputs))
	unique := make([]UsageEnvelope, 0, len(inputs))
	reason := ""
	for _, input := range inputs {
		if input.RunID == "" || input.CallID == "" {
			reason = UsageReasonIdentityMissing
			unique = append(unique, input)
			continue
		}
		identity := callIdentity{runID: input.RunID, callID: input.CallID}
		if prior, ok := seen[identity]; ok {
			if !sameActualUsage(prior, input) {
				if reason == "" {
					reason = UsageReasonDuplicateCallConflict
				}
			}
			continue
		}
		seen[identity] = input
		unique = append(unique, input)
	}
	return unique, reason
}

func taskIdentityFromUsage(usage []UsageEnvelope) string {
	for _, envelope := range usage {
		if envelope.TaskID != "" {
			return envelope.TaskID
		}
	}
	return ""
}

func percentReduction(before, after float64) float64 {
	return math.Round(((before-after)/before*100)*1000) / 1000
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}
