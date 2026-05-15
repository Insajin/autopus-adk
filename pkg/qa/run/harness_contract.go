package run

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	qaproject "github.com/insajin/autopus-adk/pkg/qa/project"
)

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] @AX:SPEC: SPEC-QAMESH-006: ADK remains the harness while target projects own concrete Journey Packs.
// @AX:REASON: Plan and run results expose this contract to agents so adapter execution, redaction, and feedback stay separated from product-specific journeys.
func harnessContract(projectDir string) HarnessContract {
	return HarnessContract{
		Role:                 "harness",
		JourneyPackOwnership: "project-local",
		JourneyPackRoot:      ".autopus/qa/journeys",
		RuntimeArtifactRoot:  ".autopus/qa/runs",
		GeneratedPolicy:      "ADK owns adapters, execution, redaction, and feedback; the target project owns concrete Journey Packs.",
		Guidance:             "Create or review project-local Journey Packs before GUI execution; do not hard-code product-specific journeys into ADK.",
	}
}

func projectLocalJourneySetupGaps(opts Options, packs []journey.Pack) []SetupGap {
	if hasGUIExplorePack(packs) || !isGUIExploreRequest(opts) {
		return nil
	}
	return []SetupGap{{
		Adapter:   "gui-explore",
		JourneyID: "project-local-gui-explore",
		Reason:    projectLocalGUIJourneyReason(),
	}}
}

func projectLocalJourneyHints(opts Options, packs []journey.Pack) []SetupGap {
	if hasGUIExplorePack(packs) || isGUIExploreRequest(opts) || !qaproject.HasDesktopGUISignals(opts.ProjectDir) {
		return nil
	}
	return []SetupGap{{
		Adapter:   "gui-explore",
		JourneyID: "project-local-gui-explore",
		Reason:    "desktop GUI tooling detected; " + projectLocalGUIJourneyReason(),
	}}
}

func hasGUIExplorePack(packs []journey.Pack) bool {
	for _, pack := range packs {
		if pack.Adapter.ID == "gui-explore" && journey.HasLane(pack, "gui-explore") {
			return true
		}
	}
	return false
}

func isGUIExploreRequest(opts Options) bool {
	if opts.AdapterID == "gui-explore" || strings.EqualFold(opts.Lane, "gui-explore") {
		return true
	}
	return false
}

func projectLocalGUIJourneyReason() string {
	return "project-local gui-explore Journey Pack required: ADK is a harness; create .autopus/qa/journeys/<id>.yaml with allowed origins, forbidden actions, deterministic oracles, and redacted artifact retention"
}
