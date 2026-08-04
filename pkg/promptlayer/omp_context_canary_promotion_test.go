package promptlayer

import (
	"fmt"
	"testing"
)

func TestEvaluateOMPContextHistoryPromotionV1_GatePriorityAndMinimumPairs(t *testing.T) {
	t.Parallel()
	base := promotionRowsV1(20, 2500)

	integrity := append([]OMPContextCanaryRowV1(nil), base...)
	integrity[0].IntegrityPassed = false
	decision := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: OMPContextHistoryModeActiveV1,
		PreviousHistoryMode:  OMPContextHistoryModeShadowV1,
		MemoryMode:           OMPContextMemoryModeOffV1,
		Rows:                 integrity,
	})
	if decision.EffectiveHistoryMode != OMPContextHistoryModeShadowV1 || decision.Reason != OMPContextPromotionReasonIntegrityMismatchV1 {
		t.Fatalf("integrity gate did not win: %+v", decision)
	}
	if len(decision.Gates) == 0 || decision.Gates[0].ID != "integrity" || decision.Gates[0].Passed {
		t.Fatalf("integrity gate ordering mismatch: %+v", decision.Gates)
	}

	insufficient := promotionRowsV1(19, 2500)
	decision = EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: OMPContextHistoryModeActiveV1,
		PreviousHistoryMode:  OMPContextHistoryModeShadowV1,
		MemoryMode:           OMPContextMemoryModeShadowV1,
		Rows:                 insufficient,
	})
	if decision.Reason != OMPContextPromotionReasonInsufficientPairCountV1 || decision.EffectiveHistoryMode != OMPContextHistoryModeShadowV1 {
		t.Fatalf("minimum pair gate mismatch: %+v", decision)
	}
	if decision.EffectiveMemoryMode != OMPContextMemoryModeShadowV1 {
		t.Fatalf("promotion changed memory mode: %+v", decision)
	}
}

func TestEvaluateOMPContextHistoryPromotionV1_ActiveOnlyWhenAllGatesPass(t *testing.T) {
	t.Parallel()
	rows := promotionRowsV1(22, 2000)
	decision := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: OMPContextHistoryModeActiveV1,
		PreviousHistoryMode:  OMPContextHistoryModeShadowV1,
		MemoryMode:           OMPContextMemoryModeOffV1,
		Rows:                 rows,
	})
	if !decision.Admitted || decision.EffectiveHistoryMode != OMPContextHistoryModeActiveV1 || decision.Reason != OMPContextPromotionReasonPassedV1 {
		t.Fatalf("valid promotion rejected: %+v", decision)
	}
	if decision.EffectiveMemoryMode != OMPContextMemoryModeOffV1 {
		t.Fatalf("history promotion changed memory mode: %+v", decision)
	}
	for _, gate := range decision.Gates {
		if !gate.Passed {
			t.Fatalf("promotion admitted with failed gate: %+v", decision.Gates)
		}
	}
}

func TestEvaluateOMPContextRollbackV1_RequiresEffectiveReadbackAndPreservesMemory(t *testing.T) {
	t.Parallel()
	activeHash := ompContextMemoryTestHash("active-overlay")
	rollbackHash := ompContextMemoryTestHash("rollback-overlay")
	userConfigHash := ompContextMemoryTestHash("user-config")
	input := OMPContextRollbackInputV1{
		RequestedHistoryMode:  OMPContextHistoryModeShadowV1,
		PreviousHistoryMode:   OMPContextHistoryModeActiveV1,
		MemoryModeBefore:      OMPContextMemoryModeOffV1,
		MemoryModeAfter:       OMPContextMemoryModeOffV1,
		ActiveOverlayHash:     activeHash,
		RollbackOverlayHash:   rollbackHash,
		EffectiveReadbackHash: rollbackHash,
		UserConfigBeforeHash:  userConfigHash,
		UserConfigAfterHash:   userConfigHash,
		TriggerReason:         OMPContextRollbackReasonQualityRegressionV1,
	}
	receipt := EvaluateOMPContextRollbackV1(input)
	if receipt.Admitted || receipt.EffectiveHistoryMode != OMPContextHistoryModeShadowV1 || receipt.Reason != OMPContextRollbackReasonCanonicalRestartUnverifiedV1 {
		t.Fatalf("overlay-only rollback did not fail closed: %+v", receipt)
	}
	if !receipt.ReadbackVerified || receipt.CanonicalFullDelivery || !receipt.OptimizedStateContinued {
		t.Fatalf("rollback claimed unproved canonical restart: %+v", receipt)
	}
	if receipt.EffectiveMemoryMode != OMPContextMemoryModeOffV1 {
		t.Fatalf("rollback changed memory mode: %+v", receipt)
	}

	input.EffectiveReadbackHash = activeHash
	failed := EvaluateOMPContextRollbackV1(input)
	if failed.Admitted || failed.Reason != OMPContextRollbackReasonReadbackMismatchV1 || failed.EffectiveHistoryMode != OMPContextHistoryModeActiveV1 {
		t.Fatalf("readback mismatch did not fail closed: %+v", failed)
	}
}

func TestEvaluateOMPContextHistoryPromotionV1_RejectsSecurityQualityAndTokenFailures(t *testing.T) {
	t.Parallel()
	base := promotionRowsV1(20, 2000)
	cases := []struct {
		name   string
		mutate func([]OMPContextCanaryRowV1)
		reason string
	}{
		{"security", func(rows []OMPContextCanaryRowV1) { rows[0].SecurityPassed = false }, OMPContextPromotionReasonSecurityFindingV1},
		{"quality", func(rows []OMPContextCanaryRowV1) { rows[1].QualityScore-- }, OMPContextPromotionReasonQualityRegressionV1},
		{"order", func(rows []OMPContextCanaryRowV1) {
			for index := 0; index < len(rows); index += 2 {
				rows[index].Order, rows[index+1].Order = 1, 2
			}
		}, OMPContextPromotionReasonUnbalancedOrderV1},
		{"token", func(rows []OMPContextCanaryRowV1) {
			for index := 1; index < len(rows); index += 2 {
				rows[index].Tokens = 8001
			}
		}, OMPContextPromotionReasonInsufficientReductionV1},
		{"fallback", func(rows []OMPContextCanaryRowV1) { rows[1].FallbackVerified = false }, OMPContextPromotionReasonFallbackUnverifiedV1},
		{"rollback", func(rows []OMPContextCanaryRowV1) { rows[1].RollbackVerified = false }, OMPContextPromotionReasonRollbackUnverifiedV1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]OMPContextCanaryRowV1(nil), base...)
			tc.mutate(rows)
			decision := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{RequestedHistoryMode: OMPContextHistoryModeActiveV1, PreviousHistoryMode: OMPContextHistoryModeShadowV1, MemoryMode: OMPContextMemoryModeOffV1, Rows: rows})
			if decision.Admitted || decision.Reason != tc.reason {
				t.Fatalf("gate reason = %q, want %q: %+v", decision.Reason, tc.reason, decision)
			}
		})
	}
}

func promotionAggregateV1(count int, reduction int64) OMPContextCanaryAggregateV1 {
	aggregate := OMPContextCanaryAggregateV1{
		SchemaVersion: OMPContextCanarySchemaV1,
		PairKeys:      []string{}, Pairs: []OMPContextCanaryPairV1{}, PairCount: count,
		ABCount: (count + 1) / 2, BACount: count / 2, BalancedOrder: true,
		MedianReductionBasisPoints: reduction, MedianReductionPercent: ompContextPercentV1(reduction),
		FallbackVerified: true, RollbackVerified: true,
	}
	for index := 0; index < count; index++ {
		key := fmt.Sprintf("T%02d", index)
		aggregate.PairKeys = append(aggregate.PairKeys, key)
		aggregate.Pairs = append(aggregate.Pairs, OMPContextCanaryPairV1{TaskID: key})
	}
	return aggregate
}

func promotionRowsV1(count int, reduction int64) []OMPContextCanaryRowV1 {
	rows := make([]OMPContextCanaryRowV1, 0, count*2)
	for index := 0; index < count; index++ {
		fullOrder, optimizedOrder := 1, 2
		if index%2 == 1 {
			fullOrder, optimizedOrder = 2, 1
		}
		key := fmt.Sprintf("T%02d", index)
		rows = append(rows,
			OMPContextCanaryRowV1{TaskID: key, Variant: OMPContextCanaryVariantFullV1, Order: fullOrder, Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100},
			OMPContextCanaryRowV1{TaskID: key, Variant: OMPContextCanaryVariantOptimizedV1, Order: optimizedOrder, Tokens: 10000 - reduction, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100, FallbackVerified: true, RollbackVerified: true},
		)
	}
	return rows
}

func TestEvaluateOMPContextHistoryPromotionV1_DefaultShadowAndInvalidMode(t *testing.T) {
	t.Parallel()
	shadow := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		PreviousHistoryMode: OMPContextHistoryModeShadowV1,
		MemoryMode:          OMPContextMemoryModeOffV1,
	})
	if !shadow.Admitted || shadow.EffectiveHistoryMode != OMPContextHistoryModeShadowV1 || shadow.Reason != "requested-shadow" {
		t.Fatalf("default shadow decision mismatch: %+v", shadow)
	}
	invalid := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: OMPContextHistoryModeActiveV1,
		PreviousHistoryMode:  "invalid",
		MemoryMode:           OMPContextMemoryModeOffV1,
	})
	if invalid.Admitted || invalid.Reason != OMPContextPromotionReasonInvalidModeV1 {
		t.Fatalf("invalid mode was admitted: %+v", invalid)
	}
}

func TestEvaluateOMPContextRollbackV1_StableFailureReasons(t *testing.T) {
	t.Parallel()
	activeHash := ompContextMemoryTestHash("active")
	rollbackHash := ompContextMemoryTestHash("rollback")
	userHash := ompContextMemoryTestHash("user")
	base := OMPContextRollbackInputV1{
		RequestedHistoryMode:  OMPContextHistoryModeOffV1,
		PreviousHistoryMode:   OMPContextHistoryModeActiveV1,
		MemoryModeBefore:      OMPContextMemoryModeShadowV1,
		MemoryModeAfter:       OMPContextMemoryModeShadowV1,
		ActiveOverlayHash:     activeHash,
		RollbackOverlayHash:   rollbackHash,
		EffectiveReadbackHash: rollbackHash,
		UserConfigBeforeHash:  userHash,
		UserConfigAfterHash:   userHash,
		TriggerReason:         OMPContextRollbackReasonOperatorRequestedV1,
	}
	cases := []struct {
		name   string
		mutate func(*OMPContextRollbackInputV1)
		reason string
		mode   OMPContextHistoryModeV1
	}{
		{"invalid", func(input *OMPContextRollbackInputV1) { input.ActiveOverlayHash = "bad" }, OMPContextRollbackReasonInvalidEvidenceV1, OMPContextHistoryModeShadowV1},
		{"memory", func(input *OMPContextRollbackInputV1) { input.MemoryModeAfter = OMPContextMemoryModeOffV1 }, OMPContextRollbackReasonMemoryChangedV1, OMPContextHistoryModeActiveV1},
		{"user config", func(input *OMPContextRollbackInputV1) {
			input.UserConfigAfterHash = ompContextMemoryTestHash("changed")
		}, OMPContextRollbackReasonUserConfigMutatedV1, OMPContextHistoryModeActiveV1},
		{"overlay", func(input *OMPContextRollbackInputV1) {
			input.RollbackOverlayHash = input.ActiveOverlayHash
			input.EffectiveReadbackHash = input.ActiveOverlayHash
		}, OMPContextRollbackReasonOverlayUnchangedV1, OMPContextHistoryModeActiveV1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.mutate(&input)
			receipt := EvaluateOMPContextRollbackV1(input)
			if receipt.Admitted || receipt.Reason != tc.reason || receipt.EffectiveHistoryMode != tc.mode {
				t.Fatalf("rollback reason = %q, want %q: %+v", receipt.Reason, tc.reason, receipt)
			}
		})
	}
}
