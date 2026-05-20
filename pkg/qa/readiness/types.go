package readiness

const (
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: readiness projection schema is the portable ADK QAMESH wire contract.
	// @AX:REASON: CLI output and downstream project consumers rely on these JSON field names and status values.
	ProjectionSchemaVersion = "qamesh.readiness_projection.v1"
	ContractOwner           = "autopus-adk"

	StatusPassed   Status = "passed"
	StatusFailed   Status = "failed"
	StatusBlocked  Status = "blocked"
	StatusSkipped  Status = "skipped"
	StatusSetupGap Status = "setup_gap"
	StatusDeferred Status = "deferred"

	ReleaseVerdictBlocked ReleaseVerdict = "blocked"
	ReleaseVerdictPassed  ReleaseVerdict = "passed"
	ReleaseVerdictWarn    ReleaseVerdict = "warn"

	RedactionPassed RedactionStatus = "passed"
	RedactionFailed RedactionStatus = "failed"
)

type Status string
type ReleaseVerdict string
type RedactionStatus string

type Input struct {
	WorkspaceRoot string
	RepoRoot      string
	WorkspaceID   string
	RepoID        string
	RunIndexPath  string
	ReleasePath   string
}

type Result struct {
	Projection           *Projection           `json:"projection,omitempty"`
	Rendered             *RenderedValues       `json:"rendered,omitempty"`
	ProviderRepairPrompt *ProviderRepairPrompt `json:"provider_repair_prompt,omitempty"`
	Blockers             []Blocker             `json:"blockers,omitempty"`
}

type Projection struct {
	SchemaVersion      string            `json:"schema_version"`
	ContractOwner      string            `json:"contract_owner"`
	ReferenceConsumers []string          `json:"reference_consumers,omitempty"`
	ReadOnly           bool              `json:"read_only"`
	ReleaseVerdict     ReleaseVerdict    `json:"release_verdict"`
	RawPayloadPresent  bool              `json:"raw_payload_present"`
	LaneStatuses       map[string]Status `json:"lane_statuses"`
	Lanes              []LaneStatus      `json:"lanes"`
	CheckCounts        CheckCounts       `json:"check_counts"`
	SetupGaps          []SetupGap        `json:"setup_gaps,omitempty"`
	DeferredLanes      []string          `json:"deferred_lanes,omitempty"`
	EvidenceRefs       []EvidenceRef     `json:"evidence_refs,omitempty"`
	FeedbackRefs       []FeedbackRef     `json:"feedback_refs,omitempty"`
	FeedbackActions    []FeedbackAction  `json:"feedback_actions,omitempty"`
	SafeActions        []SafeAction      `json:"safe_actions,omitempty"`
	AuditRefs          []string          `json:"audit_refs,omitempty"`
	SourceChains       []SourceChain     `json:"source_chains,omitempty"`
	RouteCandidates    []RouteCandidate  `json:"route_candidates,omitempty"`
	LastRunTime        string            `json:"last_run_time,omitempty"`
	TrendSummary       string            `json:"trend_summary,omitempty"`
}

type LaneStatus struct {
	Lane   string `json:"lane"`
	Status Status `json:"status"`
}

type CheckCounts struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Blocked int `json:"blocked,omitempty"`
}

type SetupGap struct {
	Lane   string `json:"lane,omitempty"`
	Class  string `json:"class"`
	Reason string `json:"reason,omitempty"`
}

type EvidenceRef struct {
	ManifestPath string `json:"manifest_path"`
	QAResultID   string `json:"qa_result_id,omitempty"`
}

type FeedbackRef struct {
	BundlePath string `json:"bundle_path"`
	Target     string `json:"target,omitempty"`
}

type SourceChain struct {
	Lane             string   `json:"lane,omitempty"`
	ReleaseIndexPath string   `json:"release_index_path,omitempty"`
	RunIndexPath     string   `json:"run_index_path,omitempty"`
	ManifestPath     string   `json:"manifest_path,omitempty"`
	FeedbackBundle   string   `json:"feedback_bundle,omitempty"`
	AuditRef         string   `json:"audit_ref,omitempty"`
	SourceRefs       []string `json:"source_refs,omitempty"`
}

type SafeAction struct {
	Action   string `json:"action"`
	ReadOnly bool   `json:"read_only"`
}

type RouteCandidate struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type RenderedValues struct {
	Fields []RenderedField `json:"fields"`
}

type RenderedField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ProviderRepairPrompt struct {
	Text string `json:"text"`
}

type Blocker struct {
	Class string `json:"class"`
}

type EvidenceForFeedback struct {
	Status                 Status
	DeterministicAuthority bool
	RedactionStatus        RedactionStatus
	ManifestPath           string
}

type FeedbackAction struct {
	Target          string   `json:"target,omitempty"`
	Enabled         bool     `json:"enabled"`
	Command         []string `json:"command,omitempty"`
	CommandDisplay  string   `json:"command_display,omitempty"`
	DisabledReason  string   `json:"disabled_reason,omitempty"`
	ManifestPath    string   `json:"manifest_path,omitempty"`
	RedactionStatus string   `json:"redaction_status,omitempty"`
}
