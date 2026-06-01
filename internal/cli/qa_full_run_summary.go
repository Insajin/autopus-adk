package cli

import qarelease "github.com/insajin/autopus-adk/pkg/qa/release"

func buildQAFullRunSummary(status string, result qarelease.ExecutionPayload, domain qaFullDomainReadiness) qaFullSummary {
	summary := qaFullSummary{
		Status:              status,
		Action:              "run",
		SelectedLanes:       result.SelectedLanes,
		JourneyPackCount:    len(result.LaneRows),
		SetupGapCount:       len(result.SetupGaps),
		BlockingSetupGaps:   countBlockingSetupGaps(result.SetupGaps),
		DomainScenarioCount: domainScenarioCount(domain),
		DomainSetupGap:      domain.Status != "ready",
	}
	annotateQAFullRootBlocker(&summary, result.LaneRows)
	return summary
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
