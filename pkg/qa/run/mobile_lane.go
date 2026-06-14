package run

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/mobile"
)

// laneMobileScripted is the executable Maestro scripted mobile lane. It reuses
// the mobile readiness gate but, unlike mobile-readiness, runs the project-local
// Maestro flow once readiness is satisfied.
const laneMobileScripted = "mobile-scripted"

// applyMobileScriptedLane mirrors applyMobileReadiness for the scripted lane.
// It assesses mobile readiness, records setup gaps, and clears the selected
// surface when readiness is not satisfied so execution stays fail-closed.
// mobileReadinessExecutionStatus keys on plan.MobileReadiness and therefore
// serves both lanes without modification.
func applyMobileScriptedLane(plan *Plan, opts Options) {
	if !strings.EqualFold(opts.Lane, laneMobileScripted) {
		return
	}
	readiness := mobile.Assess(opts.ProjectDir)
	plan.MobileReadiness = &readiness
	for _, gap := range readiness.SetupGaps {
		plan.SetupGaps = append(plan.SetupGaps, SetupGap{
			Adapter: laneMobileScripted,
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
