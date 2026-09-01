package cli

import (
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

func buildQAFullRunSummary(status string, result qarelease.ExecutionPayload, domain qaFullDomainReadiness, journeyPackCount int) qaFullSummary {
	summary := qaFullSummary{
		Status:              status,
		Action:              "run",
		SelectedLanes:       result.SelectedLanes,
		JourneyPackCount:    journeyPackCount,
		SetupGapCount:       len(result.SetupGaps),
		BlockingSetupGaps:   countBlockingSetupGaps(result.SetupGaps),
		DomainScenarioCount: domainScenarioCount(domain),
		DomainSetupGap:      domain.Status != "ready",
	}
	annotateQAFullRootBlocker(&summary, result.LaneRows)
	return summary
}

// countProjectJourneyPacks counts the Journey Packs the project declares.
//
// The release execution index carries lane rows, not packs, and the run summary
// used to report len(LaneRows) under the journey_pack_count name - a fixed
// seven-lane matrix that inflated a two-pack project's apparent coverage 3.5x.
// The packs on disk are the same source BuildPlan counts in plan mode, so both
// modes now answer the same question.
//
// An unreadable pack set counts as zero: the release gate that just ran loads
// the same directory and fails loudly on it, so this path never has to invent a
// number to cover for it.
func countProjectJourneyPacks(projectDir string) int {
	packs, err := journey.LoadDir(projectDir)
	if err != nil {
		return 0
	}
	return len(packs)
}

func annotateQAFullRootBlocker(summary *qaFullSummary, rows []qarelease.LaneRow) {
	for _, row := range rows {
		if row.LaneVerdict != qarelease.LaneVerdictBlock {
			continue
		}
		summary.RootBlockerLane = row.Lane
		if len(row.Blockers) > 0 {
			summary.RootBlockerReason = row.Blockers[0].Reason
		}
		summary.RootFailedJourneyID = row.FailedJourneyID
		summary.RootFailureSummary = row.FailureSummary
		return
	}
}
