package release

import (
	"errors"
	"time"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: schema version strings are persisted in release plan, index, roadmap, and blocker matrix JSON.
const (
	PlanSchemaVersion     = "qamesh.release_plan.v1"
	IndexSchemaVersion    = "qamesh.release_index.v1"
	RoadmapSchemaVersion  = "qamesh.release_roadmap.v1"
	BlockingMatrixVersion = "qamesh.release_blocking.v1"
)

var (
	ErrInvalidProfile = errors.New("invalid qa release profile")
	ErrReleaseBlocked = errors.New("qa release blocked")
)

type Options struct {
	ProjectDir    string
	Profile       string
	Output        string
	RunOutputRoot string
	Command       string
	DryRun        bool
	Roadmap       bool
	Runner        LaneRunner
	Now           func() time.Time
	NewID         func() string
}

type LaneRunner interface {
	RunLane(opts Options, lane string) (LaneRunResult, error)
}

type LaneRunnerFunc func(opts Options, lane string) (LaneRunResult, error)

func (fn LaneRunnerFunc) RunLane(opts Options, lane string) (LaneRunResult, error) {
	return fn(opts, lane)
}

type LaneRunResult struct {
	Status          LaneStatus      `json:"status"`
	RunIndexPath    string          `json:"run_index_path,omitempty"`
	ManifestPaths   []string        `json:"manifest_paths,omitempty"`
	FeedbackRefs    []string        `json:"feedback_refs,omitempty"`
	AIAnalysisRefs  []AIAnalysisRef `json:"ai_analysis_refs,omitempty"`
	RedactionStatus RedactionState  `json:"redaction_status,omitempty"`
}

type RedactionState string

const (
	RedactionClean    RedactionState = "clean"
	RedactionRedacted RedactionState = "redacted"
	RedactionBlocked  RedactionState = "blocked"
)

type LanePolicy string

const (
	LanePolicyMust     LanePolicy = "must"
	LanePolicyOptional LanePolicy = "optional"
	LanePolicyDeferred LanePolicy = "deferred"
)

type LaneStatus string

const (
	LaneStatusPassed   LaneStatus = "passed"
	LaneStatusWarn     LaneStatus = "warn"
	LaneStatusWarning  LaneStatus = "warning"
	LaneStatusFailed   LaneStatus = "failed"
	LaneStatusBlocked  LaneStatus = "blocked"
	LaneStatusSetupGap LaneStatus = "setup_gap"
	LaneStatusDeferred LaneStatus = "deferred"
	LaneStatusSkipped  LaneStatus = "skipped"
)

type SetupGapClass string

const (
	SetupGapNone               SetupGapClass = "none"
	SetupGapMissingJourneyPack SetupGapClass = "missing-journey-pack"
	SetupGapCanaryTemplate     SetupGapClass = "canary-template"
	SetupGapToolUnavailable    SetupGapClass = "tool-unavailable"
	SetupGapEnvMissing         SetupGapClass = "env-missing"
	SetupGapSiblingSpecPending SetupGapClass = "sibling-spec-pending"
	SetupGapPolicyForbidden    SetupGapClass = "policy-forbidden"
	SetupGapUnsafeCommand      SetupGapClass = "unsafe-command"
)

type Severity string

const (
	SeverityNone     Severity = "none"
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type LaneVerdict string

const (
	LaneVerdictPass  LaneVerdict = "pass"
	LaneVerdictWarn  LaneVerdict = "warn"
	LaneVerdictBlock LaneVerdict = "block"
)

type GateStatus string

const (
	GateStatusPassed  GateStatus = "passed"
	GateStatusWarn    GateStatus = "warn"
	GateStatusBlocked GateStatus = "blocked"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-004: release payload structs form the CLI JSON output contract.
// @AX:REASON: auto qa release, tests, prompt guidance, and downstream evidence tooling consume these field names and versioned envelopes.
type Plan struct {
	SchemaVersion   string           `json:"schema_version"`
	Command         string           `json:"command"`
	DryRun          bool             `json:"dry_run"`
	Profile         string           `json:"profile"`
	LaneCatalog     []LaneCatalogRow `json:"lane_catalog"`
	SelectedLanes   []string         `json:"selected_lanes"`
	JourneyPacks    []JourneyPackRow `json:"journey_packs"`
	SetupGaps       []SetupGapRow    `json:"setup_gaps"`
	BlockerRules    BlockerRules     `json:"blocker_rules"`
	OutputPaths     OutputPaths      `json:"output_paths"`
	SiblingSpecs    []SiblingSpec    `json:"sibling_specs"`
	RedactionStatus RedactionState   `json:"redaction_status"`
	RedactionRules  []string         `json:"redaction_rules"`
	SideEffects     []string         `json:"side_effects"`
}

type LaneCatalogRow struct {
	Lane                string   `json:"lane"`
	OwnerSpec           string   `json:"owner_spec"`
	OwnerRepo           string   `json:"owner_repo"`
	ReadinessContract   string   `json:"readiness_contract"`
	ImplementationState string   `json:"implementation_state"`
	SupportedProfiles   []string `json:"supported_profiles"`
}

type JourneyPackRow struct {
	Lane                   string   `json:"lane"`
	JourneyID              string   `json:"journey_id"`
	Adapter                string   `json:"adapter"`
	Source                 string   `json:"source"`
	CommandDeclared        bool     `json:"command_declared"`
	CommandPreview         string   `json:"command_preview"`
	CommandPreviewRedacted bool     `json:"command_preview_redacted"`
	Executable             bool     `json:"executable"`
	SourceSpec             string   `json:"source_spec"`
	AcceptanceRefs         []string `json:"acceptance_refs"`
	InventedCommand        bool     `json:"invented_command"`
}

type SetupGapRow struct {
	Lane            string        `json:"lane"`
	SetupGapClass   SetupGapClass `json:"setup_gap_class"`
	Reason          string        `json:"reason"`
	Severity        Severity      `json:"severity"`
	Blocking        bool          `json:"blocking"`
	OwnerSpec       string        `json:"owner_spec"`
	OwnerRepo       string        `json:"owner_repo"`
	InventedCommand bool          `json:"invented_command"`
}

type BlockerRules struct {
	Profile       string           `json:"profile"`
	MustLanes     []string         `json:"must_lanes"`
	OptionalLanes []string         `json:"optional_lanes"`
	DeferredLanes []string         `json:"deferred_lanes"`
	SeverityOrder []Severity       `json:"severity_order"`
	MatrixVersion string           `json:"matrix_version"`
	RuleRows      []BlockerRuleRow `json:"rule_rows"`
}

type BlockerRuleRow struct {
	LanePolicy    LanePolicy  `json:"lane_policy"`
	LaneStatus    string      `json:"lane_status"`
	SetupGapClass string      `json:"setup_gap_class"`
	Severity      string      `json:"severity"`
	LaneVerdict   LaneVerdict `json:"lane_verdict"`
	GateEffect    string      `json:"gate_effect"`
}

type OutputPaths struct {
	ReleaseIndexPreviewPath string `json:"release_index_preview_path,omitempty"`
	ReleaseIndexPath        string `json:"release_index_path,omitempty"`
	RunIndexRoot            string `json:"run_index_root"`
	EvidenceRoot            string `json:"evidence_root"`
	FeedbackRoot            string `json:"feedback_root"`
}

type SiblingSpec struct {
	SpecID       string   `json:"spec_id"`
	OwnerRepo    string   `json:"owner_repo"`
	Lanes        []string `json:"lanes"`
	Status       string   `json:"status"`
	Relationship string   `json:"relationship"`
}

type Index struct {
	SchemaVersion          string          `json:"schema_version"`
	ReleaseID              string          `json:"release_id"`
	Profile                string          `json:"profile"`
	StartedAt              string          `json:"started_at"`
	EndedAt                string          `json:"ended_at"`
	Status                 GateStatus      `json:"status"`
	SelectedLanes          []string        `json:"selected_lanes"`
	LaneRows               []LaneRow       `json:"lane_rows"`
	SetupGaps              []SetupGapRow   `json:"setup_gaps"`
	Blockers               []Blocker       `json:"blockers"`
	OutputPaths            OutputPaths     `json:"output_paths"`
	SiblingSpecs           []SiblingSpec   `json:"sibling_specs"`
	FeedbackRefs           []string        `json:"feedback_refs"`
	AIAnalysisRefs         []AIAnalysisRef `json:"ai_analysis_refs"`
	DeterministicAuthority bool            `json:"deterministic_authority"`
	RedactionStatus        RedactionState  `json:"redaction_status"`
}

type ExecutionPayload struct {
	Index
	ReleaseIndexPath string `json:"release_index_path"`
}

type LaneRow struct {
	Lane                   string        `json:"lane"`
	LanePolicy             LanePolicy    `json:"lane_policy"`
	OwnerSpec              string        `json:"owner_spec"`
	OwnerRepo              string        `json:"owner_repo"`
	Status                 LaneStatus    `json:"status"`
	SetupGapClass          SetupGapClass `json:"setup_gap_class"`
	Severity               Severity      `json:"severity"`
	LaneVerdict            LaneVerdict   `json:"lane_verdict"`
	RunIndexPath           string        `json:"run_index_path"`
	ManifestPaths          []string      `json:"manifest_paths"`
	FeedbackRefs           []string      `json:"feedback_refs"`
	Blockers               []Blocker     `json:"blockers"`
	SkippedReason          string        `json:"skipped_reason"`
	DeterministicAuthority bool          `json:"deterministic_authority"`
}

type Blocker struct {
	Lane   string `json:"lane,omitempty"`
	Reason string `json:"reason"`
}

type AIAnalysisRef struct {
	Ref               string `json:"ref"`
	TrustedForVerdict bool   `json:"trusted_for_verdict"`
}

type RoadmapPayload struct {
	SchemaVersion string                   `json:"schema_version"`
	GeneratedAt   string                   `json:"generated_at"`
	Lanes         []RoadmapLane            `json:"lanes"`
	SiblingSpecs  []SiblingSpec            `json:"sibling_specs"`
	Profiles      map[string]ProfilePolicy `json:"profiles"`
}

type RoadmapLane struct {
	Lane                 string                `json:"lane"`
	OwnerSpec            string                `json:"owner_spec"`
	OwnerRepo            string                `json:"owner_repo"`
	ImplementationState  string                `json:"implementation_state"`
	SiblingDependency    string                `json:"sibling_dependency"`
	ReadinessContract    string                `json:"readiness_contract"`
	LanePolicyByProfile  map[string]LanePolicy `json:"lane_policy_by_profile"`
	LaunchBlockingPolicy map[string]LanePolicy `json:"launch_blocking_policy"`
}

type ProfilePolicy struct {
	MustLanes     []string `json:"must_lanes"`
	OptionalLanes []string `json:"optional_lanes"`
	DeferredLanes []string `json:"deferred_lanes"`
}
