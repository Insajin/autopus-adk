package delivery

import "errors"

const (
	ExecutionContractV1       = "codeops.execution.v1"
	DeliveryDoctorSchemaV1    = "codeops.delivery_doctor.v1"
	DeliveryPreparationSchema = "codeops.delivery_preparation.v1"
)

const (
	ReasonScopeInvalid    = "delivery_scope_invalid"
	ReasonHarnessInvalid  = "delivery_harness_invalid"
	ReasonContractInvalid = "delivery_contract_invalid"
)

type ConvergenceError struct {
	code string
}

func (e *ConvergenceError) Error() string { return "delivery convergence validation failed" }
func (e *ConvergenceError) Code() string  { return e.code }

func convergenceError(code string) error { return &ConvergenceError{code: code} }

func ConvergenceReasonCode(err error) string {
	var typed *ConvergenceError
	if errors.As(err, &typed) {
		return typed.Code()
	}
	return ReasonContractInvalid
}

type DoctorOptions struct {
	WorkingDirectory string
	RepoScopeRef     string
	Phase            Phase
}

type DoctorReceipt struct {
	SchemaVersion  string `json:"schema_version"`
	Status         string `json:"status"`
	RepoScopeRef   string `json:"repo_scope_ref"`
	Phase          Phase  `json:"phase"`
	ScopedWorktree bool   `json:"scoped_worktree"`
	HarnessDigest  string `json:"harness_digest"`
	ContextDigest  string `json:"context_digest"`
}

type ExecutionContract struct {
	ContractVersion         string `json:"contract_version"`
	RequestID               string `json:"request_id"`
	ExecutionID             string `json:"execution_id"`
	WorkspaceID             string `json:"workspace_id"`
	RuntimeInstanceID       string `json:"runtime_instance_id"`
	RepoConnectionID        string `json:"repo_connection_id"`
	RepoScopeRef            string `json:"repo_scope_ref"`
	Phase                   Phase  `json:"phase"`
	Attempt                 int    `json:"attempt"`
	LeaseID                 string `json:"lease_id"`
	LeaseExpiresAt          string `json:"lease_expires_at"`
	CorrelationID           string `json:"correlation_id,omitempty"`
	LeaseIssuedAt           string `json:"lease_issued_at,omitempty"`
	Objective               string `json:"objective"`
	ExpectedResultSchema    string `json:"expected_result_schema"`
	ExecutionContractDigest string `json:"execution_contract_digest"`
}

type PhasePreparation struct {
	ContractVersion         string `json:"contract_version"`
	RequestID               string `json:"request_id"`
	ExecutionID             string `json:"execution_id"`
	WorkspaceID             string `json:"workspace_id"`
	RuntimeInstanceID       string `json:"runtime_instance_id"`
	RepoConnectionID        string `json:"repo_connection_id"`
	RepoScopeRef            string `json:"repo_scope_ref"`
	Phase                   Phase  `json:"phase"`
	Attempt                 int    `json:"attempt"`
	LeaseID                 string `json:"lease_id"`
	LeaseExpiresAt          string `json:"lease_expires_at"`
	CorrelationID           string `json:"correlation_id,omitempty"`
	LeaseIssuedAt           string `json:"lease_issued_at,omitempty"`
	Prompt                  string `json:"prompt"`
	ExpectedResultSchema    string `json:"expected_result_schema"`
	ExecutionContractDigest string `json:"execution_contract_digest"`
	HarnessDigest           string `json:"harness_digest"`
	ContextDigest           string `json:"context_digest"`
}

type Preparation struct {
	SchemaVersion           string             `json:"schema_version"`
	RepoScopeRef            string             `json:"repo_scope_ref"`
	Phase                   Phase              `json:"phase"`
	ExecutionContractDigest string             `json:"execution_contract_digest"`
	PreparationDigest       string             `json:"preparation_digest"`
	PhaseContracts          []PhasePreparation `json:"phase_contracts"`
}
