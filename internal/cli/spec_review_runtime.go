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
	specReviewBackendFactory  = orchestra.NewSubprocessBackendImpl
)

// shippedStatuses lists spec statuses that represent work already delivered.
// A PASS verdict from a fresh review must never silently regress these back
// to `approved` (issue #38).
var shippedStatuses = map[string]struct{}{
	"completed":   {},
	"implemented": {},
}

func syncReviewedSpecStatus(specDir string, result *spec.ReviewResult) error {
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

	return spec.UpdateStatus(specDir, "approved")
}
