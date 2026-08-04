package promptlayer

import "reflect"

func EvaluateOMPContextHistoryPromotionV1(input OMPContextHistoryPromotionInputV1) OMPContextHistoryPromotionDecisionV1 {
	requested := input.RequestedHistoryMode
	if requested == "" {
		requested = OMPContextHistoryModeShadowV1
	}
	decision := OMPContextHistoryPromotionDecisionV1{
		RequestedHistoryMode: OMPContextHistoryModeShadowV1,
		EffectiveHistoryMode: OMPContextHistoryModeShadowV1,
		PreviousHistoryMode:  OMPContextHistoryModeShadowV1,
		RequestedMemoryMode:  OMPContextMemoryModeOffV1,
		EffectiveMemoryMode:  OMPContextMemoryModeOffV1,
		Gates:                []OMPContextPromotionGateV1{},
	}
	if !validOMPContextHistoryModeV1(requested) || !validOMPContextHistoryModeV1(input.PreviousHistoryMode) || !validOMPContextMemoryModeV1(input.MemoryMode) {
		decision.Reason = OMPContextPromotionReasonInvalidModeV1
		return decision
	}
	decision.RequestedHistoryMode = requested
	decision.PreviousHistoryMode = input.PreviousHistoryMode
	decision.RequestedMemoryMode = input.MemoryMode
	decision.EffectiveMemoryMode = input.MemoryMode
	if requested != OMPContextHistoryModeActiveV1 {
		decision.EffectiveHistoryMode = requested
		decision.Admitted = true
		decision.Reason = "requested-" + string(requested)
		return decision
	}
	if len(input.Rows) == 0 {
		decision.Reason = OMPContextPromotionReasonEvidenceUnverifiableV1
		decision.Gates = []OMPContextPromotionGateV1{{ID: "evidence", Passed: false, Reason: decision.Reason}}
		return decision
	}
	aggregate, err := ReduceOMPContextCanaryPairsV1(input.Rows)
	if err != nil || (!reflect.DeepEqual(input.Aggregate, OMPContextCanaryAggregateV1{}) && !reflect.DeepEqual(input.Aggregate, aggregate)) {
		decision.Reason = OMPContextPromotionReasonEvidenceUnverifiableV1
		decision.Gates = []OMPContextPromotionGateV1{{ID: "evidence", Passed: false, Reason: decision.Reason}}
		return decision
	}
	return evaluateOMPContextHistoryPromotionAggregateV1(requested, input.PreviousHistoryMode, input.MemoryMode, aggregate)
}

func evaluateOMPContextHistoryPromotionAggregateV1(
	requested OMPContextHistoryModeV1,
	previous OMPContextHistoryModeV1,
	memory OMPContextMemoryModeV1,
	aggregate OMPContextCanaryAggregateV1,
) OMPContextHistoryPromotionDecisionV1 {
	decision := OMPContextHistoryPromotionDecisionV1{
		RequestedHistoryMode: requested, EffectiveHistoryMode: OMPContextHistoryModeShadowV1,
		PreviousHistoryMode: previous, RequestedMemoryMode: memory, EffectiveMemoryMode: memory,
		Gates: []OMPContextPromotionGateV1{},
	}
	exactPairs := aggregate.InvalidRows == 0 && aggregate.DuplicateRows == 0 && aggregate.UnpairedRows == 0 &&
		aggregate.PairCount == len(aggregate.Pairs) && aggregate.PairCount == len(aggregate.PairKeys)
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-003: promotion requires 20 paired runs and at least 20% median token reduction.
	decision.Gates = []OMPContextPromotionGateV1{
		{ID: "integrity", Passed: exactPairs && aggregate.IntegrityFailures == 0, Reason: OMPContextPromotionReasonIntegrityMismatchV1},
		{ID: "security", Passed: aggregate.SecurityFailures == 0, Reason: OMPContextPromotionReasonSecurityFindingV1},
		{ID: "quality", Passed: aggregate.QualityRegressions == 0, Reason: OMPContextPromotionReasonQualityRegressionV1},
		{ID: "pair-count", Passed: aggregate.PairCount >= 20, Reason: OMPContextPromotionReasonInsufficientPairCountV1},
		{ID: "order-balance", Passed: aggregate.BalancedOrder, Reason: OMPContextPromotionReasonUnbalancedOrderV1},
		{ID: "token-reduction", Passed: aggregate.MedianReductionBasisPoints >= 2000, Reason: OMPContextPromotionReasonInsufficientReductionV1},
		{ID: "fallback", Passed: aggregate.FallbackVerified, Reason: OMPContextPromotionReasonFallbackUnverifiedV1},
		{ID: "rollback", Passed: aggregate.RollbackVerified, Reason: OMPContextPromotionReasonRollbackUnverifiedV1},
	}
	for _, gate := range decision.Gates {
		if !gate.Passed {
			decision.Reason = gate.Reason
			return decision
		}
	}
	decision.EffectiveHistoryMode = requested
	decision.Admitted = true
	decision.Reason = OMPContextPromotionReasonPassedV1
	return decision
}

// @AX:WARN [AUTO]: rollback evaluation has cyclomatic complexity 16.
// @AX:REASON [AUTO]: gocyclo reports 16 across promotion state, policy digest, regression, and terminal-decision branches.
func EvaluateOMPContextRollbackV1(input OMPContextRollbackInputV1) OMPContextRollbackReceiptV1 {
	receipt := OMPContextRollbackReceiptV1{
		RequestedHistoryMode:    OMPContextHistoryModeShadowV1,
		EffectiveHistoryMode:    OMPContextHistoryModeShadowV1,
		PreviousHistoryMode:     OMPContextHistoryModeShadowV1,
		EffectiveMemoryMode:     OMPContextMemoryModeOffV1,
		OptimizedStateContinued: true,
	}
	if input.PreviousHistoryMode != OMPContextHistoryModeActiveV1 ||
		(input.RequestedHistoryMode != OMPContextHistoryModeShadowV1 && input.RequestedHistoryMode != OMPContextHistoryModeOffV1) ||
		!validOMPContextMemoryModeV1(input.MemoryModeBefore) || !validOMPContextMemoryModeV1(input.MemoryModeAfter) ||
		!validOMPContextCanaryHashV1(input.ActiveOverlayHash) || !validOMPContextCanaryHashV1(input.RollbackOverlayHash) ||
		!validOMPContextCanaryHashV1(input.EffectiveReadbackHash) || !validOMPContextCanaryHashV1(input.UserConfigBeforeHash) ||
		!validOMPContextCanaryHashV1(input.UserConfigAfterHash) || !validOMPContextRollbackTriggerV1(input.TriggerReason) {
		receipt.Reason = OMPContextRollbackReasonInvalidEvidenceV1
		return receipt
	}
	receipt.RequestedHistoryMode = input.RequestedHistoryMode
	receipt.EffectiveHistoryMode = input.PreviousHistoryMode
	receipt.PreviousHistoryMode = input.PreviousHistoryMode
	receipt.EffectiveMemoryMode = input.MemoryModeBefore
	receipt.ActiveOverlayHash = input.ActiveOverlayHash
	receipt.RollbackOverlayHash = input.RollbackOverlayHash
	receipt.EffectiveReadbackHash = input.EffectiveReadbackHash
	if input.MemoryModeBefore != input.MemoryModeAfter {
		receipt.Reason = OMPContextRollbackReasonMemoryChangedV1
		return receipt
	}
	if input.UserConfigBeforeHash != input.UserConfigAfterHash {
		receipt.Reason = OMPContextRollbackReasonUserConfigMutatedV1
		return receipt
	}
	if input.ActiveOverlayHash == input.RollbackOverlayHash {
		receipt.Reason = OMPContextRollbackReasonOverlayUnchangedV1
		return receipt
	}
	if input.EffectiveReadbackHash != input.RollbackOverlayHash {
		receipt.Reason = OMPContextRollbackReasonReadbackMismatchV1
		return receipt
	}
	receipt.EffectiveHistoryMode = input.RequestedHistoryMode
	receipt.ReadbackVerified = true
	receipt.Reason = OMPContextRollbackReasonCanonicalRestartUnverifiedV1
	return receipt
}

func validOMPContextHistoryModeV1(mode OMPContextHistoryModeV1) bool {
	return mode == OMPContextHistoryModeOffV1 || mode == OMPContextHistoryModeShadowV1 || mode == OMPContextHistoryModeActiveV1
}

func validOMPContextMemoryModeV1(mode OMPContextMemoryModeV1) bool {
	return mode == OMPContextMemoryModeOffV1 || mode == OMPContextMemoryModeShadowV1
}

func validOMPContextCanaryHashV1(value string) bool {
	return validOMPContextMemoryHashV1(value)
}

func validOMPContextRollbackTriggerV1(reason string) bool {
	return reason == OMPContextRollbackReasonQualityRegressionV1 || reason == OMPContextRollbackReasonIntegrityRegressionV1 ||
		reason == OMPContextRollbackReasonSecurityRegressionV1 || reason == OMPContextRollbackReasonOperatorRequestedV1
}
