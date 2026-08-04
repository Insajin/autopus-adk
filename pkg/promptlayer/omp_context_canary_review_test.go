package promptlayer

import (
	"testing"
	"time"
)

func TestEvaluateOMPContextHistoryPromotionV1_RejectsCallerAggregateWithoutRawEvidence(t *testing.T) {
	t.Parallel()

	decision := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: OMPContextHistoryModeActiveV1,
		PreviousHistoryMode:  OMPContextHistoryModeShadowV1,
		MemoryMode:           OMPContextMemoryModeOffV1,
		Aggregate:            promotionAggregateV1(20, 2500),
	})

	if decision.Admitted || decision.EffectiveHistoryMode != OMPContextHistoryModeShadowV1 ||
		decision.Reason != OMPContextPromotionReasonEvidenceUnverifiableV1 {
		t.Fatalf("caller aggregate admitted active mode: %+v", decision)
	}
}

func TestEvaluateOMPContextHistoryPromotionV1_RejectsAggregateThatDisagreesWithRawRows(t *testing.T) {
	t.Parallel()
	rows := reviewPromotionRows(20)
	aggregate, err := ReduceOMPContextCanaryPairsV1(rows)
	if err != nil {
		t.Fatalf("reduce fixture: %v", err)
	}
	aggregate.MedianReductionBasisPoints = 9900

	decision := EvaluateOMPContextHistoryPromotionV1(OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: OMPContextHistoryModeActiveV1, PreviousHistoryMode: OMPContextHistoryModeShadowV1,
		MemoryMode: OMPContextMemoryModeOffV1, Rows: rows, Aggregate: aggregate,
	})

	if decision.Admitted || decision.Reason != OMPContextPromotionReasonEvidenceUnverifiableV1 {
		t.Fatalf("manipulated aggregate was trusted: %+v", decision)
	}
}

func TestOMPContextPromotionAttestationV1_BindsRawRowsPolicyAndSession(t *testing.T) {
	t.Parallel()
	checkedAt := time.Date(2026, 8, 2, 1, 0, 0, 0, time.UTC)
	input := OMPContextPromotionAttestationInputV1{
		Subject: OMPContextPromotionSubjectV1{
			WorkspaceID: "workspace", SpecID: "SPEC-OMP-004", TaskID: "TASK-7", Phase: "go",
			SessionID: "session-1", BindingHash: reviewCanaryHash("binding"),
		},
		Policy: OMPContextPromotionPolicyV1{
			Profile: "active", HistoryMode: "active", MemoryMode: "off", HistoryTargetTokens: 1000,
			Fallback: "canonical_full", CapabilityPolicy: "probe_required",
			RuntimeRootPolicy: "isolated_task_owned", MutationScope: "session_overlay",
		},
		Rows: reviewPromotionRows(20), CheckedAt: checkedAt, ValidFor: time.Hour,
	}
	attestation, err := BuildOMPContextPromotionAttestationV1(input)
	if err != nil {
		t.Fatalf("build attestation: %v", err)
	}
	if err := VerifyOMPContextPromotionAttestationV1(attestation, input.Subject, input.Policy, input.Rows, checkedAt.Add(30*time.Minute)); err != nil {
		t.Fatalf("verify attestation: %v", err)
	}

	tampered := append([]OMPContextCanaryRowV1(nil), input.Rows...)
	tampered[1].Tokens--
	if err := VerifyOMPContextPromotionAttestationV1(attestation, input.Subject, input.Policy, tampered, checkedAt.Add(30*time.Minute)); err == nil {
		t.Fatal("tampered raw canary rows were accepted")
	}
	mismatchedPolicy := input.Policy
	mismatchedPolicy.HistoryTargetTokens++
	if err := VerifyOMPContextPromotionAttestationV1(attestation, input.Subject, mismatchedPolicy, input.Rows, checkedAt.Add(30*time.Minute)); err == nil {
		t.Fatal("mismatched policy was accepted")
	}
	if err := VerifyOMPContextPromotionAttestationV1(attestation, input.Subject, input.Policy, input.Rows, checkedAt.Add(2*time.Hour)); err == nil {
		t.Fatal("stale promotion attestation was accepted")
	}
}

func TestEvaluateOMPContextRollbackV1_DoesNotClaimUnprovedCanonicalRestart(t *testing.T) {
	t.Parallel()
	input := validReviewRollbackInput()

	receipt := EvaluateOMPContextRollbackV1(input)

	if !receipt.ReadbackVerified {
		t.Fatalf("valid overlay rollback readback was not retained: %+v", receipt)
	}
	if receipt.CanonicalFullDelivery || !receipt.OptimizedStateContinued || receipt.Admitted {
		t.Fatalf("overlay-only rollback forged canonical restart evidence: %+v", receipt)
	}
	if receipt.Reason != OMPContextRollbackReasonCanonicalRestartUnverifiedV1 {
		t.Fatalf("rollback reason = %q", receipt.Reason)
	}
}

func reviewPromotionRows(pairCount int) []OMPContextCanaryRowV1 {
	rows := make([]OMPContextCanaryRowV1, 0, pairCount*2)
	for i := 0; i < pairCount; i++ {
		fullOrder, optimizedOrder := 1, 2
		if i%2 == 1 {
			fullOrder, optimizedOrder = 2, 1
		}
		taskID := "task-" + reviewCanaryIndex(i)
		rows = append(rows,
			OMPContextCanaryRowV1{TaskID: taskID, Variant: OMPContextCanaryVariantFullV1, Order: fullOrder, Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100},
			OMPContextCanaryRowV1{TaskID: taskID, Variant: OMPContextCanaryVariantOptimizedV1, Order: optimizedOrder, Tokens: 7500, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100, FallbackVerified: true, RollbackVerified: true},
		)
	}
	return rows
}

func reviewCanaryIndex(value int) string {
	const digits = "0123456789"
	if value < 10 {
		return "0" + string(digits[value])
	}
	return string(digits[value/10]) + string(digits[value%10])
}

func reviewCanaryHash(seed string) string {
	switch seed {
	case "binding":
		return "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	case "active":
		return "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	case "rollback":
		return "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	default:
		return "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	}
}

func validReviewRollbackInput() OMPContextRollbackInputV1 {
	return OMPContextRollbackInputV1{
		RequestedHistoryMode: OMPContextHistoryModeShadowV1, PreviousHistoryMode: OMPContextHistoryModeActiveV1,
		MemoryModeBefore: OMPContextMemoryModeOffV1, MemoryModeAfter: OMPContextMemoryModeOffV1,
		ActiveOverlayHash: reviewCanaryHash("active"), RollbackOverlayHash: reviewCanaryHash("rollback"),
		EffectiveReadbackHash: reviewCanaryHash("rollback"), UserConfigBeforeHash: reviewCanaryHash("config"),
		UserConfigAfterHash: reviewCanaryHash("config"), TriggerReason: OMPContextRollbackReasonQualityRegressionV1,
	}
}
