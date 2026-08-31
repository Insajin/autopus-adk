package report

import (
	"sort"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	"github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

// journeyViews projects every ingested manifest into a journey card and adds a
// placeholder for any adapter result whose manifest never made it through
// ingestion, so no executed journey silently disappears from the report.
func journeyViews(in ingestion, embedMedia bool) ([]JourneyView, Timeline) {
	budget := newPreviewBudget()
	media := newMediaBudget(embedMedia)
	failures := adapterFailures(in)
	views := make([]JourneyView, 0, len(in.manifests))
	ingested := make(map[string]bool, len(in.manifests))
	for _, item := range in.manifests {
		view := journeyView(in.projectRoot, item, budget, media)
		if summary, ok := failures[view.JourneyID]; ok && view.FailureSummary == "" {
			view.FailureSummary = summary
		}
		ingested[view.JourneyID] = true
		views = append(views, view)
	}
	views = append(views, missingJourneys(in, ingested)...)
	sort.SliceStable(views, func(a, b int) bool {
		// Journeys without a parseable start time carry no position on the
		// timeline, so they sort after every timestamped journey.
		left, right := views[a].StartedAt, views[b].StartedAt
		if (left == "") != (right == "") {
			return right == ""
		}
		if left != right {
			return left < right
		}
		return views[a].JourneyID < views[b].JourneyID
	})
	return applyTimeline(views)
}

func journeyView(root string, item ingestedManifest, budget *previewBudget, media *mediaBudget) JourneyView {
	manifest := item.manifest
	view := JourneyView{
		JourneyID:           journeyID(manifest),
		StepID:              manifest.SourceRefs.StepID,
		Adapter:             firstNonEmpty(manifest.SourceRefs.Adapter, manifest.Runner.Name),
		Surface:             orUnknown(manifest.Surface),
		Lane:                orUnknown(manifest.Lane),
		Verdict:             orUnknown(manifest.Status),
		ScenarioRef:         manifest.ScenarioRef,
		QAResultID:          manifest.QAResultID,
		SchemaVersion:       manifest.SchemaVersion,
		StartedAt:           manifest.StartedAt,
		EndedAt:             manifest.EndedAt,
		DurationMS:          durationMS(manifest),
		RunnerName:          orUnknown(manifest.Runner.Name),
		RunnerCommand:       evidence.RedactText(manifest.Runner.Command),
		RunnerVersion:       manifest.Runner.Version,
		ReproductionCommand: evidence.RedactText(manifest.ReproductionCommand),
		RetentionClass:      orUnknown(manifest.RetentionClass),
		RepairPromptRef:     indexRefDisplay(root, manifest.RepairPromptRef),
		ManifestRef:         item.ref,
		Checks:              checkViews(manifest.OracleResults.Checks),
		Artifacts:           artifactViews(root, manifest.Artifacts, budget),
		Source:              sourceView(manifest.SourceRefs),
		A11y:                a11yView(manifest.OracleResults.A11y),
		Desktop:             desktopView(manifest.OracleResults),
	}
	view.FailureSummary = firstFailureSummary(view.Checks)
	attachCapture(&view, item, budget, media)
	return view
}

func journeyID(manifest evidence.Manifest) string {
	return firstNonEmpty(manifest.SourceRefs.JourneyID, manifest.ScenarioRef, manifest.QAResultID, "unknown")
}

func durationMS(manifest evidence.Manifest) int64 {
	if manifest.DurationMS > 0 {
		return manifest.DurationMS
	}
	return spanMS(manifest.StartedAt, manifest.EndedAt)
}

func checkViews(checks []evidence.CheckResult) []CheckView {
	if len(checks) == 0 {
		return nil
	}
	views := make([]CheckView, 0, len(checks))
	for _, check := range checks {
		views = append(views, CheckView{
			ID:             orUnknown(check.ID),
			Type:           orUnknown(check.Type),
			Status:         orUnknown(check.Status),
			Expected:       evidence.RedactText(check.Expected),
			Actual:         evidence.RedactText(check.Actual),
			FailureSummary: evidence.RedactText(check.FailureSummary),
			ArtifactRefs:   check.ArtifactRefs,
		})
	}
	return views
}

func sourceView(refs evidence.SourceRefs) SourceView {
	view := SourceView{
		SourceSpec:       refs.SourceSpec,
		AcceptanceRefs:   refs.AcceptanceRefs,
		OwnedPaths:       refs.OwnedPaths,
		DoNotModifyPaths: refs.DoNotModifyPaths,
	}
	if refs.Mobile != nil {
		view.MobileFlowID = refs.Mobile.FlowID
		view.MobileDigest = refs.Mobile.AppArtifactDigest
		view.MobileDeviceRef = refs.Mobile.DeviceRef
	}
	return view
}

func a11yView(oracle *evidence.A11yOracle) *A11yView {
	if oracle == nil {
		return nil
	}
	return &A11yView{
		CriticalCount: oracle.CriticalCount,
		SeriousCount:  oracle.SeriousCount,
		FailedTargets: oracle.FailedTargets,
	}
}

func desktopView(results evidence.OracleResults) *DesktopView {
	observation := results.DesktopObservation
	if observation == nil {
		if results.Desktop == nil {
			return nil
		}
		return &DesktopView{TimeoutClass: results.Desktop.TimeoutClassification}
	}
	view := &DesktopView{CheckCount: len(observation.DeterministicChecks)}
	if results.Desktop != nil {
		view.TimeoutClass = results.Desktop.TimeoutClassification
	}
	if projection := observation.SemanticProjection; projection != nil {
		view.SchemaVersion = projection.SchemaVersion
		view.ProviderRef = projection.ProviderRef
		view.AppRef = projection.AppRef
		view.WindowRef = projection.WindowRef
		view.StateRef = projection.StateRef
		view.Digest = projection.Digest
		view.RootRole = string(projection.Root.Role)
		view.NodeCount = countNodes(projection.Root)
	}
	return view
}

func countNodes(node desktopobserve.SemanticNode) int {
	total := 1
	for _, child := range node.Children {
		total += countNodes(child)
	}
	return total
}

// adapterFailures indexes run-index failure summaries by journey so a manifest
// without its own failing check still shows why the lane failed.
func adapterFailures(in ingestion) map[string]string {
	failures := map[string]string{}
	if !in.trusted || in.runIndex == nil {
		return failures
	}
	for _, result := range in.runIndex.AdapterResults {
		if result.FailureSummary != "" {
			failures[result.JourneyID] = evidence.RedactText(result.FailureSummary)
		}
	}
	return failures
}

// missingJourneys reports adapter results that produced no ingestible manifest
// and were not already explained by a setup gap.
func missingJourneys(in ingestion, ingested map[string]bool) []JourneyView {
	if !in.trusted || in.runIndex == nil {
		return nil
	}
	var views []JourneyView
	for _, result := range in.runIndex.AdapterResults {
		if ingested[result.JourneyID] || result.SetupGap != nil {
			continue
		}
		views = append(views, JourneyView{
			JourneyID:      firstNonEmpty(result.JourneyID, "unknown"),
			Adapter:        orUnknown(result.Adapter),
			Surface:        "unknown",
			Lane:           laneOf(in.runIndex),
			Verdict:        VerdictBlocked,
			RunnerName:     orUnknown(result.Adapter),
			RetentionClass: "unknown",
			FailureSummary: firstNonEmpty(evidence.RedactText(result.FailureSummary), "evidence manifest was not ingested"),
			ManifestRef:    indexRefDisplay(in.projectRoot, result.QAMESHManifestPath),
		})
	}
	return views
}

func laneOf(index *qarun.Index) string {
	if index == nil {
		return "unknown"
	}
	return orUnknown(index.Lane)
}

func firstFailureSummary(checks []CheckView) string {
	for _, check := range checks {
		if check.FailureSummary != "" {
			return check.FailureSummary
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
