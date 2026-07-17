package cli

import (
	"fmt"
	"strings"

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
	if result == nil {
		return nil
	}
	if result.Verdict != spec.VerdictPass || hasActiveFindings(result.Findings) {
		return nil
	}

	// Guard against status regression: a PASS review on a SPEC that is
	// already completed/implemented must not rewrite its status.
	doc, err := spec.Load(specDir)
	if err != nil {
		return fmt.Errorf("status gate: load spec: %w", err)
	}
	if _, shipped := shippedStatuses[strings.ToLower(doc.Status)]; shipped {
		return nil
	}

	// SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-PROMO-06: a clean PASS auto-promotes
	// only when every auxiliary document was fully observed and the provider
	// quorum was met. Degraded observation blocks promotion unless the operator
	// passed --allow-degraded, which promotes via a recorded audit override.
	decision := evaluateIntegrityGate(result.DegradedReasons, allowDegraded)
	if !decision.promote {
		// Leave the prior status unchanged and surface why plus the remedy.
		fmt.Println(formatIntegrityBlockMessage(decision.reasons))
		return nil
	}
	if decision.viaOverride {
		result.OverridePromotion = true
		fmt.Println(formatIntegrityOverrideAudit(decision.reasons))
		// Re-persist review.md so the override audit line (rendered by
		// review_persist from OverridePromotion) lands in the artifact.
		if err := spec.PersistReview(specDir, result); err != nil {
			return fmt.Errorf("status gate: persist override audit: %w", err)
		}
	}

	return spec.UpdateStatus(specDir, "approved")
}
