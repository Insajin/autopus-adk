package report

import (
	"fmt"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

// Build ingests QAMESH evidence and projects it into the report view model.
// It fails only when no run index can be located or read; every other problem
// becomes an ingestion rejection so the report still states what is missing.
func Build(opts Options) (Report, error) {
	in, err := load(opts)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   nowRFC3339(opts.Now),
		Title:         reportTitle(opts, in),
		Ingestion: Ingestion{
			Status:          ingestionStatus(in),
			RunIndexRef:     displayPath(in.projectRoot, absOrRaw(in.runIndexPath)),
			ReleaseIndexRef: displayPath(in.projectRoot, absOrRaw(in.releaseIndexPath)),
			ManifestCount:   len(in.manifests),
			Rejections:      in.rejections,
		},
		Run:     runView(in),
		Release: releaseView(in),
	}
	report.Journeys, report.Timeline = journeyViews(in, opts.EmbedMedia)
	report.SetupGaps = setupGaps(in)
	report.Retention = reportRetention(report.Journeys)
	report.Summary = summarize(report)
	report.Verdict = verdict(report, in)
	report.NextCommands = nextCommands(report)
	return report, nil
}

func nowRFC3339(now func() time.Time) string {
	if now == nil {
		now = time.Now
	}
	return now().UTC().Format(time.RFC3339)
}

func reportTitle(opts Options, in ingestion) string {
	if opts.Title != "" {
		return opts.Title
	}
	if in.runIndex != nil && in.runIndex.RunID != "" {
		return "QAMESH QA Report — " + in.runIndex.RunID
	}
	return "QAMESH QA Report"
}

func ingestionStatus(in ingestion) string {
	if !in.trusted || len(in.manifests) == 0 && len(in.rejections) > 0 {
		return IngestionBlocked
	}
	if len(in.rejections) > 0 {
		return IngestionDegraded
	}
	return IngestionComplete
}

func runView(in ingestion) *RunView {
	if in.runIndex == nil {
		return nil
	}
	index := in.runIndex
	return &RunView{
		RunID:           index.RunID,
		Status:          orUnknown(index.Status),
		Profile:         index.Profile,
		Lane:            index.Lane,
		StartedAt:       index.StartedAt,
		EndedAt:         index.EndedAt,
		DurationMS:      spanMS(index.StartedAt, index.EndedAt),
		WorkspaceID:     index.Workspace.WorkspaceID,
		RepoID:          index.Workspace.RepoID,
		RedactionStatus: orUnknown(index.RedactionStatus.Status),
		SourceRefs:      trustedOnly(in, index.SourceRefs),
		FeedbackRefs:    trustedOnly(in, indexRefDisplays(in.projectRoot, index.FeedbackBundlePaths)),
	}
}

func releaseView(in ingestion) *ReleaseView {
	if in.releaseIndex == nil {
		return nil
	}
	index := in.releaseIndex
	view := &ReleaseView{
		ReleaseID:              index.ReleaseID,
		Profile:                index.Profile,
		Status:                 string(index.Status),
		StartedAt:              index.StartedAt,
		EndedAt:                index.EndedAt,
		DeterministicAuthority: index.DeterministicAuthority,
		RedactionStatus:        orUnknown(string(index.RedactionStatus)),
	}
	for _, row := range index.LaneRows {
		view.Lanes = append(view.Lanes, laneView(row))
	}
	for _, blocker := range index.Blockers {
		view.Blockers = append(view.Blockers, blockerLabel(blocker))
	}
	for _, spec := range index.SiblingSpecs {
		view.SiblingSpecs = append(view.SiblingSpecs, fmt.Sprintf("%s (%s) %s", spec.SpecID, spec.OwnerRepo, spec.Status))
	}
	return view
}

func laneView(row qarelease.LaneRow) LaneView {
	return LaneView{
		Lane:            row.Lane,
		Policy:          string(row.LanePolicy),
		OwnerSpec:       row.OwnerSpec,
		OwnerRepo:       row.OwnerRepo,
		Status:          orUnknown(string(row.Status)),
		Verdict:         orUnknown(string(row.LaneVerdict)),
		Severity:        orUnknown(string(row.Severity)),
		SetupGapClass:   orUnknown(string(row.SetupGapClass)),
		SkippedReason:   evidence.RedactText(row.SkippedReason),
		FailedJourneyID: row.FailedJourneyID,
		FailureSummary:  evidence.RedactText(row.FailureSummary),
		ManifestCount:   len(row.ManifestPaths),
		FeedbackCount:   len(row.FeedbackRefs),
	}
}

func blockerLabel(blocker qarelease.Blocker) string {
	if blocker.Lane == "" {
		return evidence.RedactText(blocker.Reason)
	}
	return blocker.Lane + ": " + evidence.RedactText(blocker.Reason)
}

func setupGaps(in ingestion) []GapView {
	var gaps []GapView
	if in.trusted && in.runIndex != nil {
		for _, gap := range in.runIndex.SetupGaps {
			gaps = append(gaps, GapView{
				Adapter:   orUnknown(gap.Adapter),
				JourneyID: gap.JourneyID,
				Reason:    evidence.RedactText(gap.Reason),
			})
		}
	}
	if in.releaseIndex != nil {
		for _, gap := range in.releaseIndex.SetupGaps {
			gaps = append(gaps, GapView{
				Adapter:   "lane:" + gap.Lane,
				JourneyID: string(gap.SetupGapClass),
				Reason:    evidence.RedactText(gap.Reason),
			})
		}
	}
	return gaps
}

func summarize(report Report) Summary {
	summary := Summary{
		JourneyCount:  len(report.Journeys),
		SetupGapCount: len(report.SetupGaps),
		DurationMS:    report.Timeline.SpanMS,
	}
	if report.Release != nil {
		summary.LaneCount = len(report.Release.Lanes)
	}
	for _, journey := range report.Journeys {
		countJourney(&summary, journey)
	}
	return summary
}

func countJourney(summary *Summary, journey JourneyView) {
	switch journey.Verdict {
	case VerdictPassed:
		summary.JourneysPassed++
	case VerdictFailed:
		summary.JourneysFailed++
	default:
		summary.JourneysOther++
	}
	summary.CheckCount += len(journey.Checks)
	for _, check := range journey.Checks {
		switch check.Status {
		case VerdictPassed:
			summary.ChecksPassed++
		case VerdictFailed:
			summary.ChecksFailed++
		default:
			summary.ChecksOther++
		}
	}
	summary.ArtifactCount += len(journey.Artifacts)
	for _, artifact := range journey.Artifacts {
		if artifact.Preview != "" {
			summary.PreviewCount++
			continue
		}
		summary.WithheldCount++
	}
	if journey.Capture != nil {
		summary.CaptureStepCount += journey.Capture.Totals.Steps
		summary.ConsoleErrors += journey.Capture.Totals.ConsoleErrors
		summary.NetworkFailures += journey.Capture.Totals.NetworkFailures
		summary.ScreenshotCount += journey.Capture.Totals.Screenshots
	}
}

func verdict(report Report, in ingestion) string {
	if report.Ingestion.Status == IngestionBlocked {
		return VerdictBlocked
	}
	if report.Summary.JourneysFailed > 0 || report.Summary.ChecksFailed > 0 {
		return VerdictFailed
	}
	if in.runIndex != nil && in.runIndex.Status == "failed" {
		return VerdictFailed
	}
	if report.Ingestion.Status == IngestionDegraded || report.Summary.JourneysOther > 0 || report.Summary.SetupGapCount > 0 {
		return VerdictPartial
	}
	if report.Summary.JourneyCount == 0 {
		return VerdictUnknown
	}
	return VerdictPassed
}

func nextCommands(report Report) []string {
	var commands []string
	for _, journey := range report.Journeys {
		if journey.Verdict == VerdictFailed && journey.RepairPromptRef != "" {
			commands = append(commands, fmt.Sprintf("auto qa feedback --to codex --evidence %s --format json", journey.ManifestRef))
			break
		}
	}
	if report.Summary.SetupGapCount > 0 {
		commands = append(commands, "auto qa init --format json")
	}
	if report.Release == nil {
		commands = append(commands, "auto qa release --dry-run --format json")
	}
	if len(report.Ingestion.Rejections) > 0 {
		commands = append(commands, "auto qa run --format json")
	}
	return commands
}

// indexRefDisplays projects index-recorded refs, which are project-root
// relative, into display refs without touching the working directory.
func indexRefDisplays(root string, refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, indexRefDisplay(root, ref))
	}
	return out
}

// trustedOnly drops index-derived content when the index itself failed a gate.
func trustedOnly(in ingestion, values []string) []string {
	if !in.trusted {
		return nil
	}
	return values
}

// absOrRaw resolves a path for display without failing: unresolvable paths fall
// through to displayPath, which redacts anything outside the project root.
func absOrRaw(path string) string {
	if path == "" {
		return ""
	}
	resolved, err := realPath(path)
	if err != nil {
		return path
	}
	return resolved
}
