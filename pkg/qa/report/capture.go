package report

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/evidence"
)

// withheldProjectedAsCapture marks the capture index artifact whose raw JSON is
// no longer inlined because the structured projection supersedes it.
const withheldProjectedAsCapture = "projected_as_capture"

// attachCapture projects the journey's declared capture index into the view
// model. Any failure records the reason and leaves Capture nil: capture evidence
// is an addition to the report, never a precondition for rendering it.
func attachCapture(view *JourneyView, item ingestedManifest, budget *previewBudget, media *mediaBudget) {
	position, ref, ok := firstCaptureArtifact(item.manifest.Artifacts)
	if !ok {
		return
	}
	index, reason := loadCaptureIndex(ref.Path)
	if reason != "" {
		view.CaptureError = reason
		return
	}
	// Media refs are relative to the journey's local capture directory, not to
	// the artifact directory: only the sanitized index is published, while the
	// raw bytes stay under the run's _raw tree and never cross the boundary.
	view.Capture = captureView(index, capture.LocalCaptureDir(filepath.Dir(item.path)), media)
	// The raw JSON dump would now duplicate the structured section, so the
	// preview is dropped and its budget returned to the remaining artifacts.
	if position < len(view.Artifacts) {
		budget.refund(view.Artifacts[position].Preview)
		view.Artifacts[position].Preview = ""
		view.Artifacts[position].Truncated = false
		view.Artifacts[position].Withheld = withheldProjectedAsCapture
	}
}

// firstCaptureArtifact returns the first declared capture index. A second one
// would be ambiguous evidence, so it is left to render as an ordinary artifact
// instead of silently overriding the projection.
func firstCaptureArtifact(refs []evidence.ArtifactRef) (int, evidence.ArtifactRef, bool) {
	for position, ref := range refs {
		if ref.Kind == capture.ArtifactKind {
			return position, ref, true
		}
	}
	return 0, evidence.ArtifactRef{}, false
}

// loadCaptureIndex applies the capture contract as the ingestion gate: strict
// decoding plus full validation. The returned reason is a stable token so the
// report states which gate refused the index.
func loadCaptureIndex(path string) (capture.Index, string) {
	info, err := os.Stat(path)
	if err != nil {
		return capture.Index{}, "missing_capture_index"
	}
	if !info.Mode().IsRegular() {
		return capture.Index{}, "capture_index_not_regular_file"
	}
	index, err := capture.LoadIndex(path)
	if err != nil {
		return capture.Index{}, "unreadable_capture_index:" + captureText(firstLine(err.Error()))
	}
	if err := capture.Validate(index); err != nil {
		return capture.Index{}, "invalid_capture_index:" + captureText(firstLine(err.Error()))
	}
	return index, ""
}

func captureView(index capture.Index, dir string, media *mediaBudget) *CaptureView {
	view := &CaptureView{
		Mode:      orUnknown(index.Mode),
		Streams:   index.Streams,
		StartedAt: index.StartedAt,
		EndedAt:   index.EndedAt,
		Totals:    CaptureTotals(index.Totals),
		Steps:     captureSteps(index, dir, media),
		Replay:    captureReplay(index.Replay),
	}
	for _, entry := range index.Media {
		// Journey-level media is trace, video, or HAR: never inlined, so it is
		// projected without a budget and stays a digest-addressed reference.
		view.Media = append(view.Media, mediaView(entry.Kind, dir, entry.MediaRef, nil))
	}
	return view
}

func captureSteps(index capture.Index, dir string, media *mediaBudget) []CaptureStepView {
	if len(index.Steps) == 0 {
		return nil
	}
	views := make([]CaptureStepView, 0, len(index.Steps))
	for _, step := range index.Steps {
		view := CaptureStepView{
			StepID:         step.StepID,
			Order:          step.Order,
			Title:          captureText(step.Title),
			Status:         orUnknown(step.Status),
			DurationMS:     stepDurationMS(step),
			ScreenRef:      captureText(step.ScreenRef),
			FailureSummary: captureText(step.FailureSummary),
			Actions:        captureActions(step.Actions),
			Console:        captureConsole(step.Console),
			Network:        captureNetwork(step.Network),
		}
		if step.Screenshot != nil {
			shot := mediaView(capture.StreamScreenshot, dir, *step.Screenshot, media)
			view.Screenshot = &shot
		}
		views = append(views, view)
	}
	applyCaptureTimeline(views, index)
	return views
}

func stepDurationMS(step capture.Step) int64 {
	if step.DurationMS > 0 {
		return step.DurationMS
	}
	return spanMS(step.StartedAt, step.EndedAt)
}

func captureActions(actions []capture.Action) []CaptureActionView {
	if len(actions) == 0 {
		return nil
	}
	views := make([]CaptureActionView, 0, len(actions))
	for _, action := range actions {
		views = append(views, CaptureActionView{
			API:        captureText(action.API),
			TargetRef:  captureText(action.TargetRef),
			DurationMS: action.DurationMS,
		})
	}
	return views
}

func captureConsole(console *capture.ConsoleSummary) *CaptureConsoleView {
	if console == nil {
		return nil
	}
	view := &CaptureConsoleView{Errors: console.Errors, Warnings: console.Warnings, Infos: console.Infos}
	for _, message := range console.Messages {
		view.Messages = append(view.Messages, CaptureConsoleMessageView{
			Severity:  orUnknown(message.Severity),
			Text:      captureText(message.Text),
			SourceRef: captureText(message.SourceRef),
		})
	}
	return view
}

func captureNetwork(network *capture.NetworkSummary) *CaptureNetworkView {
	if network == nil {
		return nil
	}
	view := &CaptureNetworkView{Requests: network.Requests, Failures: network.Failures}
	for _, entry := range network.Entries {
		view.Entries = append(view.Entries, CaptureNetworkEntryView{
			Method:       orUnknown(entry.Method),
			URLRef:       captureText(entry.URLRef),
			Status:       entry.Status,
			ResourceType: entry.ResourceType,
			DurationMS:   entry.DurationMS,
			Bytes:        entry.Bytes,
		})
	}
	return view
}

func captureReplay(replay *capture.Replay) *CaptureReplayView {
	if replay == nil {
		return nil
	}
	return &CaptureReplayView{
		Kind:        orUnknown(replay.Kind),
		Command:     joinCommand(replay.Command),
		CommandArgs: replay.Command,
		SpecRefs:    replay.SpecRefs,
		Digest:      replay.Digest,
		StepCount:   replay.StepCount,
	}
}

// joinCommand renders the pinned argv as one display line. The contract already
// rejected shell metacharacters, so quoting only has to keep an argument that
// contains spaces readable as a single token.
func joinCommand(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		text := captureText(arg)
		if strings.ContainsAny(text, " \t") {
			text = `"` + text + `"`
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

// captureText re-applies the publication boundary to producer text. The
// published index was already sanitized, but the report is the last place the
// bytes can be stopped, and a hand-written index never passed that gate.
func captureText(value string) string {
	if value == "" {
		return ""
	}
	return evidence.RedactText(sanitizeText(value))
}

// reportRetention downgrades the whole report as soon as one raw local image was
// inlined: the document then carries evidence that never passed publication.
func reportRetention(journeys []JourneyView) string {
	for _, journey := range journeys {
		if journey.Capture != nil && hasEmbeddedMedia(journey.Capture) {
			return RetentionLocalOnly
		}
	}
	return RetentionShareable
}

func hasEmbeddedMedia(view *CaptureView) bool {
	for _, step := range view.Steps {
		if step.Screenshot != nil && step.Screenshot.Embedded {
			return true
		}
	}
	for _, entry := range view.Media {
		if entry.Embedded {
			return true
		}
	}
	return false
}
