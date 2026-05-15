package run

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/mobile"
)

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] @AX:SPEC: SPEC-QAMESH-006: mobile-readiness reports setup gaps instead of falling back to executable mobile journeys.
// @AX:REASON: BuildPlan, run index previews, and adapter selection depend on this gate to prevent mobile execution until readiness is redaction-safe and complete.
func applyMobileReadiness(plan *Plan, opts Options) {
	if !strings.EqualFold(opts.Lane, "mobile-readiness") {
		return
	}
	readiness := mobile.Assess(opts.ProjectDir)
	plan.MobileReadiness = &readiness
	for _, gap := range readiness.SetupGaps {
		plan.SetupGaps = append(plan.SetupGaps, SetupGap{
			Adapter: "mobile-readiness",
			Reason:  gap.ReasonCode + ": " + gap.Message,
		})
	}
	if readiness.Status != mobile.StatusReady {
		plan.SelectedAdapters = []string{}
		plan.SelectedJourneys = []string{}
		plan.ManifestOutputPreviewPaths = []string{}
		plan.ArtifactPreviewRefs = []ArtifactPreview{}
	}
}

func mobileReadinessExecutionStatus(plan Plan) (bool, string) {
	if plan.MobileReadiness == nil || plan.MobileReadiness.Status == mobile.StatusReady {
		return false, ""
	}
	if plan.MobileReadiness.Status == mobile.StatusDeferred {
		return true, "warning"
	}
	return true, "blocked"
}
