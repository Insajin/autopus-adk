// Package capture defines the typed GUI capture contract a project-local
// producer emits after a browser journey runs.
//
// The harness never drives Playwright itself: the project owns the command, and
// this index is the declared, validated handoff. It replaces the previous
// convention of five magic artifact kinds (journey_graph, console_summary,
// network_summary, aria_snapshot, screenshot_quarantine_ref) with per-step
// evidence that can be ordered, cross-referenced, and rendered as a filmstrip.
//
// Raw media (screenshots, traces, videos, HARs) never becomes publishable
// evidence. The index carries a digest-addressed reference plus size and
// dimensions; the bytes stay in the local capture directory.
package capture

// IndexSchemaVersion is the producer contract version. Producers must emit it
// verbatim; an unknown version is rejected rather than best-effort parsed.
const IndexSchemaVersion = "qamesh.gui_capture_index.v1"

// IndexFileName is the file the harness reads from the capture directory.
const IndexFileName = "capture-index.json"

// PublishedFileName is the sanitized projection the harness writes back for
// publication as evidence.
const PublishedFileName = "capture-index.published.json"

// ArtifactKind is the evidence artifact kind for the sanitized projection.
const ArtifactKind = "capture_index"

// DirName is the capture subdirectory the harness allocates inside a journey's
// raw run directory. RawDirName is that raw root.
const (
	DirName    = "capture"
	RawDirName = "_raw"
)

// RetentionLocalOnly is the only retention value a media reference may declare.
const RetentionLocalOnly = "local_only"

// Capture modes, mirroring journey.GUICapturePolicy.Mode.
const (
	ModeOff       = "off"
	ModeOnFailure = "on-failure"
	ModeAlways    = "always"
)

// Capture streams a producer may declare.
const (
	StreamScreenshot = "screenshot"
	StreamConsole    = "console"
	StreamNetwork    = "network"
	StreamTrace      = "trace"
	StreamVideo      = "video"
)

// Index is the producer-authored capture contract for one journey.
type Index struct {
	SchemaVersion string   `json:"schema_version"`
	JourneyID     string   `json:"journey_id"`
	Mode          string   `json:"mode"`
	Streams       []string `json:"streams"`
	StartedAt     string   `json:"started_at"`
	EndedAt       string   `json:"ended_at"`
	Steps         []Step   `json:"steps"`
	Media         []Media  `json:"media,omitempty"`
	Replay        *Replay  `json:"replay,omitempty"`
	Totals        Totals   `json:"totals"`
}

// Step is one ordered journey step. Order starts at 1 and increases by one, so
// a gap means the producer dropped evidence and the contract fails closed.
type Step struct {
	StepID         string          `json:"step_id"`
	Order          int             `json:"order"`
	Title          string          `json:"title,omitempty"`
	Status         string          `json:"status"`
	StartedAt      string          `json:"started_at,omitempty"`
	EndedAt        string          `json:"ended_at,omitempty"`
	DurationMS     int64           `json:"duration_ms,omitempty"`
	ScreenRef      string          `json:"screen_ref,omitempty"`
	FailureSummary string          `json:"failure_summary,omitempty"`
	Actions        []Action        `json:"actions,omitempty"`
	Screenshot     *MediaRef       `json:"screenshot,omitempty"`
	Console        *ConsoleSummary `json:"console,omitempty"`
	Network        *NetworkSummary `json:"network,omitempty"`
}

// Action is one recorded runner API call, used to explain what a step did
// without shipping the raw trace.
type Action struct {
	API        string `json:"api"`
	TargetRef  string `json:"target_ref,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

// MediaRef addresses a local capture file by digest. Ref is relative to the
// capture directory so the reference stays portable and leaks no local path.
type MediaRef struct {
	Ref       string `json:"ref"`
	Digest    string `json:"digest"`
	Bytes     int64  `json:"bytes"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Retention string `json:"retention"`
}

// Media is a journey-level capture file such as a trace, video, or HAR.
type Media struct {
	MediaRef
	Kind   string `json:"kind"`
	StepID string `json:"step_id,omitempty"`
}

// ConsoleSummary carries per-step console counters and retained messages.
type ConsoleSummary struct {
	Errors   int              `json:"errors"`
	Warnings int              `json:"warnings"`
	Infos    int              `json:"infos,omitempty"`
	Messages []ConsoleMessage `json:"messages,omitempty"`
}

// ConsoleMessage is one retained console or page error line.
type ConsoleMessage struct {
	Severity  string `json:"severity"`
	Text      string `json:"text"`
	SourceRef string `json:"source_ref,omitempty"`
}

// NetworkSummary carries per-step request counters and retained entries.
type NetworkSummary struct {
	Requests int            `json:"requests"`
	Failures int            `json:"failures"`
	Entries  []NetworkEntry `json:"entries,omitempty"`
}

// NetworkEntry is one request. URLRef is deliberately not a URL: it is an
// origin-relative path or an `origin:<n><path>` reference, so credentials,
// tokens, and query secrets cannot ride along.
type NetworkEntry struct {
	Method       string `json:"method"`
	URLRef       string `json:"url_ref"`
	Status       int    `json:"status"`
	ResourceType string `json:"resource_type,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	Bytes        int64  `json:"bytes,omitempty"`
}

// Replay is a deterministic re-run reference. It is a pinned command plus spec
// digests rather than synthesized code, so replaying costs no inference and
// cannot drift from the specs that actually ran.
type Replay struct {
	Kind      string   `json:"kind"`
	Command   []string `json:"command"`
	SpecRefs  []string `json:"spec_refs"`
	Digest    string   `json:"digest"`
	StepCount int      `json:"step_count,omitempty"`
}

// ReplayKindPlaywrightGrep re-runs the exact captured specs by name.
const ReplayKindPlaywrightGrep = "playwright-grep"

// Totals lets a consumer render headline numbers without walking every step.
type Totals struct {
	Steps           int   `json:"steps"`
	Actions         int   `json:"actions"`
	ConsoleErrors   int   `json:"console_errors"`
	NetworkFailures int   `json:"network_failures"`
	Screenshots     int   `json:"screenshots"`
	MediaBytes      int64 `json:"media_bytes"`
}

// Step statuses, matching the evidence check status vocabulary.
const (
	StatusPassed  = "passed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
	StatusBlocked = "blocked"
)

// Console severities, ordered least to most severe by consoleSeverityRank.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)
