package delivery

const (
	WorkflowMode         = "adk_supervised_delivery"
	PhaseResultSchemaV1  = "codeops.phase_result.v1"
	DryRunPlanSchemaV1   = "codeops.delivery_plan.v1"
	GateDecisionSchemaV1 = "codeops.phase_gate.v1"
	DefaultRetryBudget   = 2
	DefaultElapsedBudget = "bounded_by_supervisor"
	DefaultOwnerAgentID  = "pm_owner"
	DefaultBrokerState   = "not_started"
	DefaultProviderRoute = "managed_interactive"
	DefaultCorrelationID = "dry-run"
)

type Phase string

const (
	PhasePlan              Phase = "plan"
	PhaseImplement         Phase = "implement"
	PhaseVerify            Phase = "verify"
	PhaseQA                Phase = "qa"
	PhaseSync              Phase = "sync"
	PhaseDeploymentHandoff Phase = "deployment_handoff"
	PhaseCanary            Phase = "canary"
)

type ProviderMode string

const (
	ProviderClaudeSubscriptionInteractive ProviderMode = "claude_subscription_interactive"
	ProviderClaudeAPIHeadless             ProviderMode = "claude_api_headless"
	ProviderCodexSubscriptionInteractive  ProviderMode = "codex_subscription_interactive"
	ProviderCodexAPIHeadless              ProviderMode = "codex_api_headless"
)

type PhaseStatus string

const (
	StatusPassed           PhaseStatus = "passed"
	StatusWarn             PhaseStatus = "warn"
	StatusFailed           PhaseStatus = "failed"
	StatusBlocked          PhaseStatus = "blocked"
	StatusApprovalRequired PhaseStatus = "approval_required"
	StatusRetryRequested   PhaseStatus = "retry_requested"
)

type RedactionStatus string

const (
	RedactionClean    RedactionStatus = "clean"
	RedactionRedacted RedactionStatus = "redacted"
	RedactionBlocked  RedactionStatus = "blocked"
)

type PhaseResultEnvelope struct {
	SchemaVersion      string          `json:"schema_version"`
	RequestID          string          `json:"request_id"`
	Phase              Phase           `json:"phase"`
	Status             PhaseStatus     `json:"status"`
	Summary            string          `json:"summary"`
	ChangedFiles       []string        `json:"changed_files"`
	TestStatus         string          `json:"test_status"`
	EvidenceRefs       []string        `json:"evidence_refs"`
	Blockers           []string        `json:"blockers"`
	NextRequiredAction string          `json:"next_required_action"`
	RedactionStatus    RedactionStatus `json:"redaction_status"`
}

type DeliveryPhasePlan struct {
	Phase              Phase        `json:"phase"`
	WorkflowMode       string       `json:"workflow_mode"`
	OwnerAgentID       string       `json:"owner_agent_id"`
	Repository         string       `json:"repository"`
	ProviderMode       ProviderMode `json:"provider_mode"`
	ProviderRoute      string       `json:"provider_route"`
	Attempt            int          `json:"attempt"`
	RetryBudget        int          `json:"retry_budget"`
	ElapsedBudget      string       `json:"elapsed_budget"`
	BrokerState        string       `json:"broker_state"`
	GateDecision       GateDecision `json:"gate_decision"`
	CorrelationID      string       `json:"correlation_id"`
	NextRequiredAction string       `json:"next_required_action"`
}

type GateDecision struct {
	SchemaVersion string   `json:"schema_version"`
	Phase         Phase    `json:"phase"`
	Status        string   `json:"status"`
	RequiredRefs  []string `json:"required_refs"`
	BlocksOnDrift bool     `json:"blocks_on_generated_runtime_drift"`
}

type DryRunPlan struct {
	SchemaVersion string              `json:"schema_version"`
	WorkflowMode  string              `json:"workflow_mode"`
	ProviderMode  ProviderMode        `json:"provider_mode"`
	Repository    string              `json:"repository"`
	Phases        []DeliveryPhasePlan `json:"phases"`
}

func CanonicalPhases() []Phase {
	return []Phase{
		PhasePlan,
		PhaseImplement,
		PhaseVerify,
		PhaseQA,
		PhaseSync,
		PhaseDeploymentHandoff,
		PhaseCanary,
	}
}
