package report

// Report retention classes. A report that inlined raw local screenshots carries
// pixels that never passed the evidence publication boundary, so the class is
// part of the projection and is rendered as a banner rather than being implied
// by the presence of a data URI.
const (
	RetentionShareable = "shareable"
	RetentionLocalOnly = "local-only"
)

// CaptureView is the projected qamesh.gui_capture_index.v1 evidence for one
// journey. It exists only when the declared capture index decoded and validated:
// a rejected index leaves JourneyView.CaptureError set and this nil, so the
// report can never present unverified capture evidence as capture evidence.
type CaptureView struct {
	Mode      string             `json:"mode"`
	Streams   []string           `json:"streams,omitempty"`
	StartedAt string             `json:"started_at,omitempty"`
	EndedAt   string             `json:"ended_at,omitempty"`
	Totals    CaptureTotals      `json:"totals"`
	Steps     []CaptureStepView  `json:"steps,omitempty"`
	Media     []CaptureMediaView `json:"media,omitempty"`
	Replay    *CaptureReplayView `json:"replay,omitempty"`
}

// CaptureTotals mirrors capture.Totals, which the contract already proved equal
// to the step evidence, so the header numbers need no recomputation here.
type CaptureTotals struct {
	Steps           int   `json:"steps"`
	Actions         int   `json:"actions"`
	ConsoleErrors   int   `json:"console_errors"`
	NetworkFailures int   `json:"network_failures"`
	Screenshots     int   `json:"screenshots"`
	MediaBytes      int64 `json:"media_bytes"`
}

// CaptureStepView is one filmstrip frame. Bar positions the step inside the
// journey's own span, independent of the report-wide timeline axis.
type CaptureStepView struct {
	StepID         string              `json:"step_id"`
	Order          int                 `json:"order"`
	Title          string              `json:"title,omitempty"`
	Status         string              `json:"status"`
	DurationMS     int64               `json:"duration_ms"`
	ScreenRef      string              `json:"screen_ref,omitempty"`
	FailureSummary string              `json:"failure_summary,omitempty"`
	Actions        []CaptureActionView `json:"actions,omitempty"`
	Screenshot     *CaptureMediaView   `json:"screenshot,omitempty"`
	Console        *CaptureConsoleView `json:"console,omitempty"`
	Network        *CaptureNetworkView `json:"network,omitempty"`
	Bar            TimelineBar         `json:"bar"`
}

// CaptureActionView is one recorded runner API call.
type CaptureActionView struct {
	API        string `json:"api"`
	TargetRef  string `json:"target_ref,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

// CaptureMediaView describes a local capture file. Embedded and DataURI are
// populated only under Options.EmbedMedia and only after the file's digest
// matched the index; MediaError names the gate that refused otherwise, so a
// missing image is never silently indistinguishable from a suppressed one.
type CaptureMediaView struct {
	Kind       string `json:"kind"`
	Ref        string `json:"ref"`
	Digest     string `json:"digest"`
	Bytes      int64  `json:"bytes"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Retention  string `json:"retention"`
	Embedded   bool   `json:"embedded"`
	DataURI    string `json:"data_uri,omitempty"`
	MediaError string `json:"media_error,omitempty"`
}

// CaptureConsoleView carries the per-step console counters and retained lines.
type CaptureConsoleView struct {
	Errors   int                         `json:"errors"`
	Warnings int                         `json:"warnings"`
	Infos    int                         `json:"infos"`
	Messages []CaptureConsoleMessageView `json:"messages,omitempty"`
}

// CaptureConsoleMessageView is one retained console or page error line.
type CaptureConsoleMessageView struct {
	Severity  string `json:"severity"`
	Text      string `json:"text"`
	SourceRef string `json:"source_ref,omitempty"`
}

// CaptureNetworkView carries the per-step request counters and retained entries.
type CaptureNetworkView struct {
	Requests int                       `json:"requests"`
	Failures int                       `json:"failures"`
	Entries  []CaptureNetworkEntryView `json:"entries,omitempty"`
}

// CaptureNetworkEntryView is one request. URLRef stays a reference, never a URL,
// exactly as the capture contract enforced it.
type CaptureNetworkEntryView struct {
	Method       string `json:"method"`
	URLRef       string `json:"url_ref"`
	Status       int    `json:"status"`
	ResourceType string `json:"resource_type,omitempty"`
	DurationMS   int64  `json:"duration_ms"`
	Bytes        int64  `json:"bytes"`
}

// CaptureReplayView is the pinned re-run reference. Command is the joined argv
// for display; CommandArgs keeps the exact argv a caller should execute.
type CaptureReplayView struct {
	Kind        string   `json:"kind"`
	Command     string   `json:"command"`
	CommandArgs []string `json:"command_args,omitempty"`
	SpecRefs    []string `json:"spec_refs,omitempty"`
	Digest      string   `json:"digest"`
	StepCount   int      `json:"step_count"`
}
