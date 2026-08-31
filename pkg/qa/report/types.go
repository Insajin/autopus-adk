// Package report projects QAMESH run and release evidence into a
// human-readable, self-contained report view model.
//
// The view model is the single source of truth for both `--format json` and the
// rendered HTML: the renderer never reads the filesystem, so anything visible in
// the report has already passed the fail-closed ingestion boundary in load.go.
package report

// SchemaVersion identifies the report projection contract. It is deliberately a
// new schema rather than an extension of qamesh.evidence.v2 or
// qamesh.run_index.v1 so report consumers never depend on producer schemas.
const SchemaVersion = "qamesh.qa_report.v1"

// Verdict values for the report as a whole and for each journey.
const (
	VerdictPassed  = "passed"
	VerdictFailed  = "failed"
	VerdictBlocked = "blocked"
	VerdictSkipped = "skipped"
	VerdictPartial = "partial"
	VerdictUnknown = "unknown"
)

// Ingestion states. Degraded means the report rendered but at least one source
// was rejected; blocked means no trustworthy evidence could be ingested.
const (
	IngestionComplete = "complete"
	IngestionDegraded = "degraded"
	IngestionBlocked  = "blocked"
)

// Report is the complete projection rendered by the HTML template.
type Report struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Title         string        `json:"title"`
	Verdict       string        `json:"verdict"`
	Retention     string        `json:"retention"`
	Ingestion     Ingestion     `json:"ingestion"`
	Run           *RunView      `json:"run,omitempty"`
	Release       *ReleaseView  `json:"release,omitempty"`
	Summary       Summary       `json:"summary"`
	Timeline      Timeline      `json:"timeline"`
	Journeys      []JourneyView `json:"journeys,omitempty"`
	SetupGaps     []GapView     `json:"setup_gaps,omitempty"`
	NextCommands  []string      `json:"next_commands,omitempty"`
}

// Ingestion records every fail-closed decision taken while reading evidence, so
// a degraded report can never be mistaken for a complete one.
type Ingestion struct {
	Status          string      `json:"status"`
	RunIndexRef     string      `json:"run_index_ref,omitempty"`
	ReleaseIndexRef string      `json:"release_index_ref,omitempty"`
	ManifestCount   int         `json:"manifest_count"`
	Rejections      []Rejection `json:"rejections,omitempty"`
}

// Rejection is one refused source, with a redacted reference and a reason.
type Rejection struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// RunView mirrors the ingested qamesh.run_index.v1 header fields.
type RunView struct {
	RunID           string   `json:"run_id"`
	Status          string   `json:"status"`
	Profile         string   `json:"profile"`
	Lane            string   `json:"lane"`
	StartedAt       string   `json:"started_at"`
	EndedAt         string   `json:"ended_at"`
	DurationMS      int64    `json:"duration_ms"`
	WorkspaceID     string   `json:"workspace_id"`
	RepoID          string   `json:"repo_id"`
	RedactionStatus string   `json:"redaction_status"`
	SourceRefs      []string `json:"source_refs,omitempty"`
	FeedbackRefs    []string `json:"feedback_refs,omitempty"`
}

// ReleaseView mirrors the release index gate aggregation.
type ReleaseView struct {
	ReleaseID              string     `json:"release_id"`
	Profile                string     `json:"profile"`
	Status                 string     `json:"status"`
	StartedAt              string     `json:"started_at"`
	EndedAt                string     `json:"ended_at"`
	DeterministicAuthority bool       `json:"deterministic_authority"`
	RedactionStatus        string     `json:"redaction_status"`
	Lanes                  []LaneView `json:"lanes,omitempty"`
	Blockers               []string   `json:"blockers,omitempty"`
	SiblingSpecs           []string   `json:"sibling_specs,omitempty"`
}

// LaneView is one release lane row in the gate matrix.
type LaneView struct {
	Lane            string `json:"lane"`
	Policy          string `json:"policy"`
	OwnerSpec       string `json:"owner_spec"`
	OwnerRepo       string `json:"owner_repo"`
	Status          string `json:"status"`
	Verdict         string `json:"verdict"`
	Severity        string `json:"severity"`
	SetupGapClass   string `json:"setup_gap_class"`
	SkippedReason   string `json:"skipped_reason,omitempty"`
	FailedJourneyID string `json:"failed_journey_id,omitempty"`
	FailureSummary  string `json:"failure_summary,omitempty"`
	ManifestCount   int    `json:"manifest_count"`
	FeedbackCount   int    `json:"feedback_count"`
}

// JourneyView is one ingested evidence manifest, presented as a journey step.
type JourneyView struct {
	JourneyID           string         `json:"journey_id"`
	StepID              string         `json:"step_id,omitempty"`
	Adapter             string         `json:"adapter,omitempty"`
	Surface             string         `json:"surface"`
	Lane                string         `json:"lane"`
	Verdict             string         `json:"verdict"`
	ScenarioRef         string         `json:"scenario_ref"`
	QAResultID          string         `json:"qa_result_id"`
	SchemaVersion       string         `json:"schema_version"`
	StartedAt           string         `json:"started_at"`
	EndedAt             string         `json:"ended_at"`
	DurationMS          int64          `json:"duration_ms"`
	RunnerName          string         `json:"runner_name"`
	RunnerCommand       string         `json:"runner_command,omitempty"`
	RunnerVersion       string         `json:"runner_version,omitempty"`
	ReproductionCommand string         `json:"reproduction_command,omitempty"`
	RetentionClass      string         `json:"retention_class"`
	RepairPromptRef     string         `json:"repair_prompt_ref,omitempty"`
	FailureSummary      string         `json:"failure_summary,omitempty"`
	ManifestRef         string         `json:"manifest_ref"`
	Checks              []CheckView    `json:"checks,omitempty"`
	Artifacts           []ArtifactView `json:"artifacts,omitempty"`
	Source              SourceView     `json:"source"`
	A11y                *A11yView      `json:"a11y,omitempty"`
	Desktop             *DesktopView   `json:"desktop,omitempty"`
	Capture             *CaptureView   `json:"capture,omitempty"`
	CaptureError        string         `json:"capture_error,omitempty"`
	Bar                 TimelineBar    `json:"bar"`
}

// CheckView is one deterministic oracle result.
type CheckView struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	Expected       string   `json:"expected,omitempty"`
	Actual         string   `json:"actual,omitempty"`
	FailureSummary string   `json:"failure_summary,omitempty"`
	ArtifactRefs   []string `json:"artifact_refs,omitempty"`
}

// ArtifactView is one evidence artifact plus an optional inlined text preview.
// Preview is populated only for publishable, text-shaped, redaction-clean files.
type ArtifactView struct {
	Kind        string `json:"kind"`
	Ref         string `json:"ref"`
	Publishable bool   `json:"publishable"`
	Redaction   string `json:"redaction"`
	Bytes       int64  `json:"bytes"`
	Preview     string `json:"preview,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
	Withheld    string `json:"withheld,omitempty"`
}

// SourceView carries provenance that must survive into the report unchanged.
type SourceView struct {
	SourceSpec       string   `json:"source_spec,omitempty"`
	AcceptanceRefs   []string `json:"acceptance_refs,omitempty"`
	OwnedPaths       []string `json:"owned_paths,omitempty"`
	DoNotModifyPaths []string `json:"do_not_modify_paths,omitempty"`
	MobileFlowID     string   `json:"mobile_flow_id,omitempty"`
	MobileDigest     string   `json:"mobile_app_artifact_digest,omitempty"`
	MobileDeviceRef  string   `json:"mobile_device_ref,omitempty"`
}

// A11yView surfaces the accessibility oracle counters.
type A11yView struct {
	CriticalCount int      `json:"critical_count"`
	SeriousCount  int      `json:"serious_count"`
	FailedTargets []string `json:"failed_targets,omitempty"`
}

// DesktopView surfaces the typed desktop observation identity without
// reproducing the accessibility tree, which is local-only evidence.
type DesktopView struct {
	ProviderRef   string `json:"provider_ref,omitempty"`
	AppRef        string `json:"app_ref,omitempty"`
	WindowRef     string `json:"window_ref,omitempty"`
	StateRef      string `json:"state_ref,omitempty"`
	Digest        string `json:"digest,omitempty"`
	NodeCount     int    `json:"node_count"`
	CheckCount    int    `json:"check_count"`
	TimeoutClass  string `json:"timeout_classification,omitempty"`
	RootRole      string `json:"root_role,omitempty"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

// Summary holds the counters shown in the report header strip.
type Summary struct {
	JourneyCount     int   `json:"journey_count"`
	JourneysPassed   int   `json:"journeys_passed"`
	JourneysFailed   int   `json:"journeys_failed"`
	JourneysOther    int   `json:"journeys_other"`
	CheckCount       int   `json:"check_count"`
	ChecksPassed     int   `json:"checks_passed"`
	ChecksFailed     int   `json:"checks_failed"`
	ChecksOther      int   `json:"checks_other"`
	ArtifactCount    int   `json:"artifact_count"`
	PreviewCount     int   `json:"preview_count"`
	WithheldCount    int   `json:"withheld_count"`
	CaptureStepCount int   `json:"capture_step_count"`
	ConsoleErrors    int   `json:"console_errors"`
	NetworkFailures  int   `json:"network_failures"`
	ScreenshotCount  int   `json:"screenshot_count"`
	SetupGapCount    int   `json:"setup_gap_count"`
	LaneCount        int   `json:"lane_count"`
	DurationMS       int64 `json:"duration_ms"`
}

// Timeline describes the shared axis the journey bars are positioned against.
type Timeline struct {
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
	SpanMS    int64  `json:"span_ms"`
}

// TimelineBar positions one journey on the timeline as CSS-ready percentages.
type TimelineBar struct {
	OffsetPercent float64 `json:"offset_percent"`
	WidthPercent  float64 `json:"width_percent"`
}

// GapView is one setup gap that blocked a lane or adapter.
type GapView struct {
	Adapter   string `json:"adapter"`
	JourneyID string `json:"journey_id,omitempty"`
	Reason    string `json:"reason"`
}
