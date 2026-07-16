package experiment

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const auditAlgorithmV1 = "sha256_mod_100_v1"

// SelectFullDepthAudit deterministically selects an audit bucket from task and
// policy hashes. The caller controls the bounded percentage.
func SelectFullDepthAudit(taskID, policyHash string, ratePercent int) (AuditSelection, error) {
	if !canonicalSHA256(taskID) || !canonicalSHA256(policyHash) {
		return AuditSelection{}, fmt.Errorf("audit selection requires canonical sha256 task and policy identities")
	}
	if ratePercent < 0 || ratePercent > 100 {
		return AuditSelection{}, fmt.Errorf("audit rate must be between 0 and 100")
	}
	digest := sha256.Sum256([]byte(taskID + "\x00" + policyHash))
	bucket := int(binary.BigEndian.Uint64(digest[:8]) % 100)
	return AuditSelection{
		Selected: bucket < ratePercent, Bucket: bucket,
		RatePercent: ratePercent, Algorithm: auditAlgorithmV1,
	}, nil
}

// BuildRolloutReceipt emits hash-only rollout evidence. Behavior-changing
// compact policy remains shadowed unless an eligible canary is explicit.
func BuildRolloutReceipt(input RolloutReceiptInput, now time.Time) RolloutReceipt {
	risk := safeRiskTier(input.RiskTier)
	receipt := RolloutReceipt{
		Version: 1, ExperimentID: digestReceiptField(input.ExperimentID),
		TaskCorpusHash: digestReceiptField(input.TaskCorpusHash), PolicyHash: digestReceiptField(input.PolicyHash), ConfigHash: digestReceiptField(input.ConfigHash),
		RecordedAt: now.UTC(), RiskTier: risk, FullDepth: true,
		AuditSelection: input.AuditSelection,
		ReceiptKind:    "shadow", Decision: "SHADOW", ActiveProfile: "full_ultra",
	}

	if input.Sensitive && !input.FullDepth {
		receipt.ReceiptKind = "rollback"
		receipt.Decision = "ROLLBACK"
		receipt.FullDepth = true
		receipt.ReasonCodes = []string{"sensitive_compact_policy"}
		return receipt
	}
	if requiresFullDepth(risk) && !input.FullDepth {
		receipt.ReceiptKind = "rollback"
		receipt.Decision = "ROLLBACK"
		receipt.FullDepth = true
		receipt.ReasonCodes = []string{"risk_requires_full_depth"}
		return receipt
	}
	if input.Promotion.RolloutDecision == "ROLLBACK" {
		receipt.ReceiptKind = "rollback"
		receipt.Decision = "ROLLBACK"
		receipt.ReasonCodes = append([]string(nil), input.Promotion.ReasonCodes...)
		return receipt
	}
	if input.ReceiptKind == "audit_sample" && input.AuditSelection.Selected {
		receipt.ReceiptKind = "audit_sample"
		receipt.Decision = "AUDIT"
		receipt.ActiveProfile = "full_ultra"
		receipt.FullDepth = true
		receipt.SelectionReason = "audit_sample"
		return receipt
	}
	if input.ReceiptKind == "canary" && input.Promotion.RolloutDecision == "ELIGIBLE_NEXT_CANARY" {
		receipt.ReceiptKind = "canary"
		receipt.Decision = "CANARY"
		if input.Sensitive || requiresFullDepth(risk) {
			receipt.SelectionReason = "risk_requires_full_depth"
			return receipt
		}
		receipt.ActiveProfile = "compact_ultra"
		receipt.FullDepth = false
		receipt.SelectionReason = "eligible_canary"
	}
	return receipt
}

func requiresFullDepth(risk string) bool {
	switch risk {
	case "high", "critical", "sensitive", "unknown":
		return true
	default:
		return false
	}
}

func safeRiskTier(risk string) string {
	switch risk {
	case "low", "medium", "high", "critical", "sensitive", "unknown":
		return risk
	default:
		return "unknown"
	}
}

// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001 — unsafe receipt identifiers are replaced by hashes rather than echoed or rejected.
func digestReceiptField(value string) string {
	if canonicalSHA256(value) {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func canonicalSHA256(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != 71 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}
