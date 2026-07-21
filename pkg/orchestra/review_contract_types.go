package orchestra

import "errors"

const (
	ReviewPrepareSchemaV1      = "review.prepare.v1"
	ReviewPreparationSchemaV1  = "review.preparation.v1"
	ReviewProviderResultSchema = "review.provider_result.v1"
	ReviewPrepareMaximumBytes  = 256 * 1024
	ReviewResultMaximumBytes   = 1024 * 1024
	ReviewFindingsMaximum      = 200
)

var ErrReviewPrepareInvalid = errors.New("review_prepare_invalid")

type ReviewPrepareContract struct {
	SchemaVersion  string               `json:"schema_version"`
	RequestID      string               `json:"request_id"`
	WorkspaceID    string               `json:"workspace_id"`
	RepoScopeRef   string               `json:"repo_scope_ref"`
	WorkItemID     string               `json:"work_item_id"`
	ReviewRunID    string               `json:"review_run_id"`
	SnapshotDigest string               `json:"snapshot_digest"`
	Role           string               `json:"role"`
	ContractDigest string               `json:"contract_digest"`
	Providers      []ReviewProviderSpec `json:"providers"`
	Bounds         ReviewPrepareBounds  `json:"bounds"`
}

type ReviewProviderSpec struct {
	AdapterID string `json:"adapter_id"`
	Model     string `json:"model"`
	Role      string `json:"role"`
}

type ReviewPrepareBounds struct {
	MaxResultBytes int `json:"max_result_bytes"`
	MaxFindings    int `json:"max_findings"`
}

type ReviewPreparation struct {
	SchemaVersion     string                      `json:"schema_version"`
	RequestID         string                      `json:"request_id"`
	WorkspaceID       string                      `json:"workspace_id"`
	RepoScopeRef      string                      `json:"repo_scope_ref"`
	WorkItemID        string                      `json:"work_item_id"`
	ReviewRunID       string                      `json:"review_run_id"`
	SnapshotDigest    string                      `json:"snapshot_digest"`
	ContractDigest    string                      `json:"contract_digest"`
	ProviderContracts []ReviewProviderPreparation `json:"provider_contracts"`
}

type ReviewProviderPreparation struct {
	AdapterID      string `json:"adapter_id"`
	Model          string `json:"model"`
	Role           string `json:"role"`
	Prompt         string `json:"prompt"`
	PromptDigest   string `json:"prompt_digest"`
	ResultSchema   string `json:"result_schema"`
	MaxResultBytes int    `json:"max_result_bytes"`
	MaxFindings    int    `json:"max_findings"`
}
