package domainreadiness

import "time"

const (
	CatalogSchemaVersion  = "qamesh.domain_readiness_catalog.v1"
	ScenarioSchemaVersion = "qamesh.domain_readiness_scenario.v1"
	PlanSchemaVersion     = "qamesh.domain_readiness_plan.v1"
	ReportSchemaVersion   = "qamesh.domain_readiness_report.v1"
	EvidenceSchemaVersion = "domain_readiness_evidence.v1"
	DefaultCatalogPath    = ".autopus/qa/domain-readiness/catalog.json"

	RedactionStatusPassed = "metadata_only"
	RedactionStatusFailed = "redaction_failed"

	RetentionClassMetadataOnly = "metadata_only"

	PassFailSupportAIOnly = "ai_only"
)

type ScenarioMode string

const (
	ScenarioModeReadFirst        ScenarioMode = "read_first"
	ScenarioModeDraftOnly        ScenarioMode = "draft_only"
	ScenarioModeProofGateOnly    ScenarioMode = "proof_gate_only"
	ScenarioModeStagingSafeShell ScenarioMode = "staging_safe_shell"
	ScenarioModeLocalSafeShell   ScenarioMode = "local_safe_shell"
	ScenarioModeContractTest     ScenarioMode = "contract_test"
	ScenarioModeGUISafeShell     ScenarioMode = "gui_safe_shell"
	ScenarioModeHardStopCheck    ScenarioMode = "hard_stop_check"
)

type MutationBoundary string

const (
	MutationBoundaryReadOnly                    MutationBoundary = "read_only"
	MutationBoundaryApprovalGateCheck           MutationBoundary = "approval_gate_check"
	MutationBoundaryHardStopCheck               MutationBoundary = "hard_stop_check"
	MutationBoundaryStagingSafeWriteOnly        MutationBoundary = "staging_safe_write_only"
	MutationBoundaryProductionMutationForbidden MutationBoundary = "production_mutation_forbidden"
)

type EvidenceFreshness string

const (
	EvidenceFreshnessCurrent               EvidenceFreshness = "current"
	EvidenceFreshnessStale                 EvidenceFreshness = "stale"
	EvidenceFreshnessMissing               EvidenceFreshness = "missing"
	EvidenceFreshnessExpired               EvidenceFreshness = "expired"
	EvidenceFreshnessRedactionFailed       EvidenceFreshness = "redaction_failed"
	EvidenceFreshnessUnsafe                EvidenceFreshness = "unsafe"
	EvidenceFreshnessCrossWorkspaceBlocked EvidenceFreshness = "cross_workspace_blocked"
)

type ScenarioResult string

const (
	ScenarioResultPassed   ScenarioResult = "passed"
	ScenarioResultFailed   ScenarioResult = "failed"
	ScenarioResultBlocked  ScenarioResult = "blocked"
	ScenarioResultSetupGap ScenarioResult = "setup_gap"
	ScenarioResultStale    ScenarioResult = "stale"
	ScenarioResultSkipped  ScenarioResult = "skipped"
	ScenarioResultRejected ScenarioResult = "rejected"
)

type UnsafeReason string

const (
	UnsafeReasonUnsafeCommand               UnsafeReason = "unsafe_command"
	UnsafeReasonInventedCommand             UnsafeReason = "invented_command"
	UnsafeReasonProductionMutationForbidden UnsafeReason = "production_mutation_forbidden"
	UnsafeReasonRawPayloadNotAllowed        UnsafeReason = "raw_payload_not_allowed"
	UnsafeReasonBroadScrapingNotAllowed     UnsafeReason = "broad_scraping_not_allowed"
	UnsafeReasonProviderWriteNotAllowed     UnsafeReason = "provider_write_not_allowed"
	UnsafeReasonCrossWorkspaceRef           UnsafeReason = "cross_workspace_ref"
	UnsafeReasonRedactionFailed             UnsafeReason = "redaction_failed"
	UnsafeReasonStaleEvidence               UnsafeReason = "stale_evidence"
	UnsafeReasonSourceEvidenceMissing       UnsafeReason = "source_evidence_missing"
)

type DomainReadinessState string

const (
	DomainReadinessStateReady    DomainReadinessState = "ready"
	DomainReadinessStatePartial  DomainReadinessState = "partial"
	DomainReadinessStateBlocked  DomainReadinessState = "blocked"
	DomainReadinessStateSetupGap DomainReadinessState = "setup_gap"
	DomainReadinessStateStale    DomainReadinessState = "stale"
	DomainReadinessStateUnsafe   DomainReadinessState = "unsafe"
	DomainReadinessStateExcluded DomainReadinessState = "excluded"
)

type Catalog struct {
	SchemaVersion   string     `json:"schema_version"`
	SuiteID         string     `json:"suite_id,omitempty"`
	RequiredDomains []string   `json:"required_domains,omitempty"`
	Scenarios       []Scenario `json:"scenarios"`
}

type Scenario struct {
	SchemaVersion             string                   `json:"schema_version"`
	ScenarioID                string                   `json:"scenario_id"`
	Domain                    string                   `json:"domain"`
	Owner                     string                   `json:"owner"`
	OwningRepo                string                   `json:"owning_repo"`
	SourceSpecRefs            []string                 `json:"source_spec_refs"`
	ScenarioMode              ScenarioMode             `json:"scenario_mode"`
	MutationBoundary          MutationBoundary         `json:"mutation_boundary"`
	FixtureOrSourceNeed       []string                 `json:"fixture_or_source_need"`
	JourneyPackRefs           []string                 `json:"journey_pack_refs"`
	QAMESHLaneRefs            []string                 `json:"qamesh_lane_refs"`
	CanaryRefs                []string                 `json:"canary_refs"`
	BackendContractTestRefs   []string                 `json:"backend_contract_test_refs"`
	FrontendTypedCardTestRefs []string                 `json:"frontend_typed_card_test_refs"`
	DesktopTypedCardTestRefs  []string                 `json:"desktop_typed_card_test_refs"`
	ExpectedEvidence          []string                 `json:"expected_evidence"`
	PassFailOracle            []string                 `json:"pass_fail_oracle"`
	FreshnessWindowHours      int                      `json:"freshness_window_hours"`
	ForbiddenActions          []string                 `json:"forbidden_actions"`
	SafeExecutionEnvironment  SafeExecutionEnvironment `json:"safe_execution_environment"`
	LaunchQualityDomain       string                   `json:"launch_quality_domain"`
	RequestedActions          []string                 `json:"requested_actions,omitempty"`
}

type SafeExecutionEnvironment struct {
	Kind             string        `json:"kind"`
	Environment      string        `json:"environment,omitempty"`
	CWD              string        `json:"cwd,omitempty"`
	Timeout          string        `json:"timeout,omitempty"`
	EnvAllowlist     []string      `json:"env_allowlist,omitempty"`
	AllowedOrigins   []string      `json:"allowed_origins,omitempty"`
	SelectorStrategy string        `json:"selector_strategy,omitempty"`
	Command          *CommandShape `json:"command,omitempty"`
}

type CommandShape struct {
	Adapter      string   `json:"adapter,omitempty"`
	Argv         []string `json:"argv,omitempty"`
	Run          string   `json:"run,omitempty"`
	CWD          string   `json:"cwd,omitempty"`
	Timeout      string   `json:"timeout,omitempty"`
	EnvAllowlist []string `json:"env_allowlist,omitempty"`
}

type ScenarioValidationResult struct {
	ScenarioID     string         `json:"scenario_id"`
	Domain         string         `json:"domain"`
	Valid          bool           `json:"valid"`
	ScenarioResult ScenarioResult `json:"scenario_result"`
	RejectReasons  []UnsafeReason `json:"reject_reasons,omitempty"`
	Findings       []string       `json:"findings,omitempty"`
	SetupGaps      []string       `json:"setup_gaps,omitempty"`
	Blockers       []string       `json:"blockers,omitempty"`
}

type CatalogValidationReport struct {
	Valid             bool                       `json:"valid"`
	ScenarioCount     int                        `json:"scenario_count"`
	CoveredDomains    []string                   `json:"covered_domains"`
	MissingDomains    []string                   `json:"missing_domains,omitempty"`
	ValidationResults []ScenarioValidationResult `json:"validation_results"`
}

type CompileOptions struct {
	ProjectDir string
	Lane       string
}

type CompileSummary struct {
	SchemaVersion     string                     `json:"schema_version"`
	ScenarioCount     int                        `json:"scenario_count"`
	CommandsExecuted  bool                       `json:"commands_executed"`
	SelectedLane      string                     `json:"selected_lane"`
	Validation        CatalogValidationReport    `json:"validation"`
	ScenarioPlans     []ScenarioPlan             `json:"scenario_plans"`
	RejectedScenarios []ScenarioValidationResult `json:"rejected_scenarios,omitempty"`
	CoveredDomains    []string                   `json:"covered_domains"`
	MissingDomains    []string                   `json:"missing_domains,omitempty"`
}

type ScenarioPlan struct {
	ScenarioID       string           `json:"scenario_id"`
	Domain           string           `json:"domain"`
	Owner            string           `json:"owner"`
	OwningRepo       string           `json:"owning_repo"`
	ScenarioMode     ScenarioMode     `json:"scenario_mode"`
	MutationBoundary MutationBoundary `json:"mutation_boundary"`
	Adapter          string           `json:"adapter,omitempty"`
	Command          *CommandShape    `json:"command,omitempty"`
	JourneyRefs      []string         `json:"journey_refs,omitempty"`
	LaneRefs         []string         `json:"lane_refs,omitempty"`
	ArtifactRefs     []string         `json:"artifact_refs,omitempty"`
	AcceptanceRefs   []string         `json:"acceptance_refs,omitempty"`
	SourceNeeds      []string         `json:"source_needs,omitempty"`
	ExpectedEvidence []string         `json:"expected_evidence,omitempty"`
	PassFailOracle   []string         `json:"pass_fail_oracle,omitempty"`
	CanaryRefs       []string         `json:"canary_refs,omitempty"`
	SetupGaps        []string         `json:"setup_gaps,omitempty"`
	RejectReasons    []UnsafeReason   `json:"reject_reasons,omitempty"`
}

type EvidenceBuildInput struct {
	SuiteID     string
	RunID       string
	WorkspaceID string
	Scenarios   []ScenarioEvidenceInput
}

type ScenarioEvidenceInput struct {
	Scenario                Scenario
	Result                  ScenarioResult
	SourceRefs              []string
	SetupGaps               []string
	Blockers                []string
	ExpectedEvidenceRefs    []string
	ActualEvidenceRefs      []string
	QAMESHRefs              []string
	CanaryRefs              []string
	BackendContractRefs     []string
	FrontendCardRefs        []string
	DesktopCardRefs         []string
	Freshness               EvidenceFreshness
	EvidenceCapturedAt      time.Time
	ProviderReadCallCount   int
	ProviderWriteCallCount  int
	RedactionStatus         string
	RetentionClass          string
	RawPayloadPresent       bool
	AuditRefs               []string
	DeterministicOracleRefs []string
	UnsafeReasons           []UnsafeReason
	PassFailSupport         string
}

type DomainReadinessEvidence struct {
	SchemaVersion          string                `json:"schema_version"`
	SuiteID                string                `json:"suite_id"`
	RunID                  string                `json:"run_id"`
	WorkspaceID            string                `json:"workspace_id"`
	Domain                 string                `json:"domain"`
	ScenarioIDs            []string              `json:"scenario_ids"`
	DomainReadinessState   DomainReadinessState  `json:"domain_readiness_state"`
	ScenarioResults        []ScenarioResultEntry `json:"scenario_results"`
	SourceNeeds            []string              `json:"source_needs"`
	SetupGaps              []string              `json:"setup_gaps"`
	Blockers               []string              `json:"blockers"`
	ExpectedEvidenceRefs   []string              `json:"expected_evidence_refs"`
	ActualEvidenceRefs     []string              `json:"actual_evidence_refs"`
	QAMESHRefs             []string              `json:"qamesh_refs"`
	CanaryRefs             []string              `json:"canary_refs"`
	BackendContractRefs    []string              `json:"backend_contract_refs"`
	FrontendCardRefs       []string              `json:"frontend_card_refs"`
	DesktopCardRefs        []string              `json:"desktop_card_refs"`
	Freshness              EvidenceFreshness     `json:"freshness"`
	DenominatorIncluded    bool                  `json:"denominator_included"`
	ExclusionReason        string                `json:"exclusion_reason"`
	Owner                  string                `json:"owner"`
	OwningRepo             string                `json:"owning_repo"`
	AuditRefs              []string              `json:"audit_refs"`
	ProviderReadCallCount  int                   `json:"provider_read_call_count"`
	ProviderWriteCallCount int                   `json:"provider_write_call_count"`
	RedactionStatus        string                `json:"redaction_status"`
	RetentionClass         string                `json:"retention_class"`
	RawPayloadPresent      bool                  `json:"raw_payload_present"`
	UnsafeReasons          []UnsafeReason        `json:"unsafe_reasons"`
}

type ScenarioResultEntry struct {
	ScenarioID             string         `json:"scenario_id"`
	ScenarioResult         ScenarioResult `json:"scenario_result"`
	ReasonCode             string         `json:"reason_code,omitempty"`
	DeterministicOracleRef string         `json:"deterministic_oracle_ref,omitempty"`
	EvidenceRefIDs         []string       `json:"evidence_ref_ids"`
	ProviderWriteCallCount int            `json:"provider_write_call_count"`
	RedactionStatus        string         `json:"redaction_status"`
	RawPayloadPresent      bool           `json:"raw_payload_present"`
}

type ReportOptions struct {
	SuiteID     string
	RunID       string
	WorkspaceID string
}

type Report struct {
	SchemaVersion string                    `json:"schema_version"`
	SuiteID       string                    `json:"suite_id"`
	RunID         string                    `json:"run_id"`
	WorkspaceID   string                    `json:"workspace_id"`
	EvidenceCount int                       `json:"evidence_count"`
	Evidence      []DomainReadinessEvidence `json:"evidence"`
}
