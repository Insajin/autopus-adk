package run

import (
	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/mobile"
)

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
	HarnessContract            HarnessContract    `json:"harness_contract"`
	ConfiguredJourneys         []string           `json:"configured_journeys"`
	DetectedAdapters           []string           `json:"detected_adapters"`
	SelectedJourneys           []string           `json:"selected_journeys"`
	SelectedAdapters           []string           `json:"selected_adapters"`
	SetupGaps                  []SetupGap         `json:"setup_gaps"`
	MobileReadiness            *mobile.Readiness  `json:"mobile_readiness,omitempty"`
	ProjectHints               []SetupGap         `json:"project_hints,omitempty"`
	Deferred                   []SetupGap         `json:"deferred,omitempty"`
	OutputRoot                 string             `json:"output_root"`
	RunIndexPreviewPath        string             `json:"run_index_preview_path"`
	ManifestOutputPreviewPaths []string           `json:"manifest_output_preview_paths"`
	AdapterMetadata            []adapter.Metadata `json:"adapter_metadata,omitempty"`
	CandidateJourneys          []CandidateJourney `json:"candidate_journeys,omitempty"`
	ArtifactPreviewRefs        []ArtifactPreview  `json:"artifact_preview_refs,omitempty"`
	DryRun                     bool               `json:"dry_run,omitempty"`
}

type HarnessContract struct {
	Role                 string `json:"role"`
	JourneyPackOwnership string `json:"journey_pack_ownership"`
	JourneyPackRoot      string `json:"journey_pack_root"`
	RuntimeArtifactRoot  string `json:"runtime_artifact_root"`
	GeneratedPolicy      string `json:"generated_policy"`
	Guidance             string `json:"guidance"`
}

type ArtifactPreview struct {
	JourneyID   string `json:"journey_id"`
	Adapter     string `json:"adapter"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Publishable bool   `json:"publishable"`
	Redaction   string `json:"redaction"`
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

type WorkspaceRef struct {
	WorkspaceID string `json:"workspace_id"`
	RepoID      string `json:"repo_id"`
	RepoRoot    string `json:"repo_root"`
}

type Result struct {
	RunID               string             `json:"run_id"`
	Status              string             `json:"status"`
	DryRun              bool               `json:"dry_run,omitempty"`
	HarnessContract     HarnessContract    `json:"harness_contract"`
	SelectedJourneys    []string           `json:"selected_journeys"`
	SelectedAdapters    []string           `json:"selected_adapters"`
	OutputRoot          string             `json:"output_root"`
	RunIndexPreviewPath string             `json:"run_index_preview_path,omitempty"`
	RunIndexPath        string             `json:"run_index_path,omitempty"`
	ManifestPreviews    []string           `json:"manifest_output_preview_paths,omitempty"`
	ArtifactPreviews    []ArtifactPreview  `json:"artifact_preview_refs,omitempty"`
	CandidateJourneys   []CandidateJourney `json:"candidate_journeys,omitempty"`
	ManifestPaths       []string           `json:"manifest_paths"`
	FailedChecks        []string           `json:"failed_checks"`
	Checks              []IndexCheck       `json:"checks,omitempty"`
	AdapterResults      []AdapterResult    `json:"adapter_results"`
	SetupGaps           []SetupGap         `json:"setup_gaps"`
	FeedbackAvailable   bool               `json:"feedback_available"`
	FeedbackBundlePaths []string           `json:"feedback_bundle_paths"`
	RedactionStatus     RedactionStatus    `json:"redaction_status"`
	Workspace           WorkspaceRef       `json:"workspace"`
	SourceRefs          []string           `json:"source_refs"`
}

type RedactionStatus struct {
	Status string `json:"status"`
}

type Index struct {
	SchemaVersion       string          `json:"schema_version"`
	RunID               string          `json:"run_id"`
	Workspace           WorkspaceRef    `json:"workspace"`
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
	SourceRefs          []string        `json:"source_refs"`
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
