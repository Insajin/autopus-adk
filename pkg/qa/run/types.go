package run

import "github.com/insajin/autopus-adk/pkg/qa/adapter"

const RunIndexSchemaVersion = "qamesh.run_index.v1"

type Options struct {
	ProjectDir string
	Profile    string
	Lane       string
	JourneyID  string
	AdapterID  string
	Output     string
	DryRun     bool
	FeedbackTo string
}

type Plan struct {
	SelectedLane               string             `json:"selected_lane"`
	ConfiguredJourneys         []string           `json:"configured_journeys"`
	DetectedAdapters           []string           `json:"detected_adapters"`
	SelectedJourneys           []string           `json:"selected_journeys"`
	SelectedAdapters           []string           `json:"selected_adapters"`
	SetupGaps                  []SetupGap         `json:"setup_gaps"`
	Deferred                   []SetupGap         `json:"deferred,omitempty"`
	OutputRoot                 string             `json:"output_root"`
	RunIndexPreviewPath        string             `json:"run_index_preview_path"`
	ManifestOutputPreviewPaths []string           `json:"manifest_output_preview_paths"`
	AdapterMetadata            []adapter.Metadata `json:"adapter_metadata,omitempty"`
	CandidateJourneys          []CandidateJourney `json:"candidate_journeys,omitempty"`
	DryRun                     bool               `json:"dry_run,omitempty"`
}

type CandidateJourney struct {
	JourneyID         string         `json:"journey_id"`
	StepID            string         `json:"step_id"`
	Adapter           string         `json:"adapter"`
	Command           []string       `json:"command,omitempty"`
	CWD               string         `json:"cwd,omitempty"`
	Timeout           string         `json:"timeout,omitempty"`
	EnvAllowlist      []string       `json:"env_allowlist,omitempty"`
	Artifacts         []string       `json:"artifacts,omitempty"`
	AcceptanceRefs    []string       `json:"acceptance_refs,omitempty"`
	Source            string         `json:"source"`
	InputSource       string         `json:"input_source,omitempty"`
	PassFailAuthority string         `json:"pass_fail_authority,omitempty"`
	OracleThresholds  map[string]any `json:"oracle_thresholds,omitempty"`
	ManualOrDeferred  bool           `json:"manual_or_deferred,omitempty"`
	ErrorCode         string         `json:"error_code,omitempty"`
}

type SetupGap struct {
	Adapter   string `json:"adapter"`
	JourneyID string `json:"journey_id,omitempty"`
	Reason    string `json:"reason"`
}

type AdapterResult struct {
	Adapter               string    `json:"adapter"`
	JourneyID             string    `json:"journey_id"`
	Status                string    `json:"status"`
	QAMESHManifestPath    string    `json:"qamesh_manifest_path"`
	RepairPromptAvailable bool      `json:"repair_prompt_available"`
	SetupGap              *SetupGap `json:"setup_gap"`
	FailureSummary        string    `json:"failure_summary"`
}

type Result struct {
	RunID               string          `json:"run_id"`
	Status              string          `json:"status"`
	DryRun              bool            `json:"dry_run,omitempty"`
	SelectedJourneys    []string        `json:"selected_journeys"`
	SelectedAdapters    []string        `json:"selected_adapters"`
	OutputRoot          string          `json:"output_root"`
	RunIndexPath        string          `json:"run_index_path,omitempty"`
	ManifestPaths       []string        `json:"manifest_paths"`
	FailedChecks        []string        `json:"failed_checks"`
	Checks              []IndexCheck    `json:"checks,omitempty"`
	AdapterResults      []AdapterResult `json:"adapter_results"`
	SetupGaps           []SetupGap      `json:"setup_gaps"`
	FeedbackAvailable   bool            `json:"feedback_available"`
	FeedbackBundlePaths []string        `json:"feedback_bundle_paths"`
	RedactionStatus     RedactionStatus `json:"redaction_status"`
}

type RedactionStatus struct {
	Status string `json:"status"`
}

type Index struct {
	SchemaVersion       string          `json:"schema_version"`
	RunID               string          `json:"run_id"`
	Status              string          `json:"status"`
	StartedAt           string          `json:"started_at"`
	EndedAt             string          `json:"ended_at"`
	Profile             string          `json:"profile"`
	Lane                string          `json:"lane"`
	ManifestPaths       []string        `json:"manifest_paths"`
	Checks              []IndexCheck    `json:"checks"`
	AdapterResults      []AdapterResult `json:"adapter_results"`
	SetupGaps           []SetupGap      `json:"setup_gaps"`
	FeedbackBundlePaths []string        `json:"feedback_bundle_paths"`
	RedactionStatus     RedactionStatus `json:"redaction_status"`
}

type IndexCheck struct {
	ID             string `json:"id"`
	JourneyID      string `json:"journey_id"`
	Adapter        string `json:"adapter"`
	Status         string `json:"status"`
	Expected       string `json:"expected"`
	Actual         string `json:"actual"`
	FailureSummary string `json:"failure_summary,omitempty"`
}
