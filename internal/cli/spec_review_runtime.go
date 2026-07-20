package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

var (
	specReviewRunOrchestra    = runStructuredSpecReviewOrchestra
	specReviewConfigProviders = buildReviewProvidersWithConfig
	// specReviewBackendFactory routes structured spec review through SelectBackend
	// (REQ-002): pane-capable terminals get the interactive pane backend, plain/CI
	// terminals get the headless subprocess backend. It accepts the full config so
	// SelectBackend can read the detected Terminal injected by spec_review_loop.go.
	specReviewBackendFactory func(orchestra.OrchestraConfig) orchestra.ExecutionBackend = orchestra.SelectBackend
)

// shippedStatuses lists spec statuses that represent work already delivered.
// A PASS verdict from a fresh review must never silently regress these back
// to `approved` (issue #38).
var shippedStatuses = map[string]struct{}{
	"completed":   {},
	"implemented": {},
}

func syncReviewedSpecStatus(specDir string, result *spec.ReviewResult, allowDegraded bool) error {
	_, err := syncReviewedSpecStatusWithReceipt(specDir, result, allowDegraded)
	return err
}

func syncReviewedSpecStatusWithReceipt(
	specDir string,
	result *spec.ReviewResult,
	allowDegraded bool,
	runtimeEvidence ...specReviewRuntimeEvidence,
) (specReviewPromotionReceipt, error) {
	evidence := specReviewRuntimeEvidence{RunID: orchestra.NewSessionID(), FinishedAt: time.Now().UTC()}
	if len(runtimeEvidence) > 0 {
		if runtimeEvidence[0].RunID != "" {
			evidence.RunID = runtimeEvidence[0].RunID
		}
		if !runtimeEvidence[0].FinishedAt.IsZero() {
			evidence.FinishedAt = runtimeEvidence[0].FinishedAt.UTC()
		}
	}
	receipt := specReviewPromotionReceipt{
		Schema: specReviewPromotionReceiptSchema, RunID: evidence.RunID,
		FinishedAt: evidence.FinishedAt.Format(time.RFC3339Nano), DegradedReasons: []string{},
		GateStatus: "blocked",
	}
	if result == nil {
		return receipt, nil
	}
	receipt.SpecID = result.SpecID
	receipt.Verdict = string(result.Verdict)
	receipt.AnalysisVerdict = string(result.Verdict)
	receipt.DegradedReasons = append([]string(nil), result.DegradedReasons...)
	receipt.CriticalVeto = hasCriticalSpecReviewVeto(result.Findings)

	doc, err := spec.Load(specDir)
	if err != nil {
		return receipt, fmt.Errorf("status gate: load spec: %w", err)
	}
	receipt.PreviousStatus = doc.Status
	receipt.CurrentStatus = doc.Status
	if result.Verdict != spec.VerdictPass || hasActiveFindings(result.Findings) {
		return receipt, nil
	}
	receipt.GateStatus = "passed"
	if len(result.DegradedReasons) > 0 {
		receipt.GateStatus = "degraded"
	}

	// Guard against status regression: a PASS review on a SPEC that is
	// already completed/implemented must not rewrite its status.
	if _, shipped := shippedStatuses[strings.ToLower(doc.Status)]; shipped {
		return receipt, nil
	}

	// SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-PROMO-06: a clean PASS auto-promotes
	// only when every auxiliary document was fully observed and the provider
	// quorum was met. Degraded observation blocks promotion unless the operator
	// passed --allow-degraded, which promotes via a recorded audit override.
	decision := evaluateIntegrityGate(result.DegradedReasons, allowDegraded)
	if !decision.promote {
		// Leave the prior status unchanged and surface why plus the remedy.
		fmt.Println(formatIntegrityBlockMessage(decision.reasons))
		return receipt, nil
	}
	if decision.viaOverride {
		result.OverridePromotion = true
		receipt.OverrideApplied = true
		receipt.GateStatus = "passed"
		fmt.Println(formatIntegrityOverrideAudit(decision.reasons))
		// Re-persist review.md so the override audit line (rendered by
		// review_persist from OverridePromotion) lands in the artifact.
		if err := spec.PersistReview(specDir, result); err != nil {
			return receipt, fmt.Errorf("status gate: persist override audit: %w", err)
		}
	}

	if err := spec.UpdateStatus(specDir, "approved"); err != nil {
		return receipt, err
	}
	receipt.CurrentStatus = "approved"
	receipt.StatusChanged = receipt.PreviousStatus != receipt.CurrentStatus
	return receipt, nil
}

func hasCriticalSpecReviewVeto(findings []spec.ReviewFinding) bool {
	for _, finding := range findings {
		if !spec.IsActiveBlockingFinding(finding) {
			continue
		}
		if strings.EqualFold(finding.Severity, "critical") &&
			(finding.Category == spec.FindingCategorySecurity || finding.Category == spec.FindingCategoryCorrectness) {
			return true
		}
	}
	return false
}
