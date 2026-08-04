package cli

import (
	"errors"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func verifyWorkflowContextPromotion(
	request WorkflowContextRuntimeRequest,
	binding promptlayer.OMPContextBindingReceipt,
	now time.Time,
) error {
	return promptlayer.VerifyOMPContextPromotionAttestationV1(
		request.Promotion.Attestation,
		workflowContextPromotionSubject(request, binding.BindingHash),
		workflowContextPromotionPolicy(request),
		request.Promotion.Rows,
		now,
	)
}

func workflowContextPromotionSubject(request WorkflowContextRuntimeRequest, bindingHash string) promptlayer.OMPContextPromotionSubjectV1 {
	return promptlayer.OMPContextPromotionSubjectV1{
		WorkspaceID: request.Binding.WorkspaceID, SpecID: request.Binding.SpecID, TaskID: request.Binding.TaskID,
		Phase: request.Binding.Phase, SessionID: request.Binding.SessionID, BindingHash: bindingHash,
	}
}

func workflowContextPromotionPolicy(request WorkflowContextRuntimeRequest) promptlayer.OMPContextPromotionPolicyV1 {
	return promptlayer.OMPContextPromotionPolicyV1{
		Profile: request.Policy.Profile, HistoryMode: request.Policy.HistoryMode, MemoryMode: request.Policy.MemoryMode,
		HistoryTargetTokens: request.Policy.HistoryTargetTokens, Fallback: request.Policy.Fallback,
		CapabilityPolicy: request.Policy.CapabilityPolicy, RuntimeRootPolicy: request.Policy.RuntimeRootPolicy,
		MutationScope: request.Policy.MutationScope,
	}
}

func workflowContextPromotionFailureReason(err error) string {
	switch {
	case errors.Is(err, promptlayer.ErrOMPContextPromotionAttestationUnavailable):
		return "promotion-attestation-absent"
	case errors.Is(err, promptlayer.ErrOMPContextPromotionAttestationStale):
		return "promotion-attestation-stale"
	case errors.Is(err, promptlayer.ErrOMPContextPromotionAttestationMismatch):
		return "promotion-attestation-mismatch"
	default:
		return "promotion-evidence-rejected"
	}
}

func populateWorkflowContextPromotionReceipt(receipt *WorkflowContextRuntimeReceipt, evidence promptlayer.OMPContextPromotionEvidenceV1) {
	receipt.PromotionAttestation = evidence.Attestation.Digest()
	receipt.PromotionPolicyDigest = evidence.Attestation.PolicyDigest()
	receipt.CanaryDigest = evidence.Attestation.CanaryDigest()
	receipt.PromotionCheckedAt = evidence.Attestation.CheckedAt().UTC().Format(time.RFC3339Nano)
}
