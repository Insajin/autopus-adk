package promptlayer

import "errors"

const OMPContextReceiptSchemaVersion = "autopus.omp_context_receipt.v1"

var (
	ErrOMPContextBodySerialization  = errors.New("OMP transient context bodies cannot be serialized")
	ErrOMPContextBindingUnavailable = errors.New("OMP context binding is unavailable")
)

type OMPContextDocumentReference struct {
	SourceRef  string `json:"source_ref"`
	SourceHash string `json:"source_hash"`
	PromptHash string `json:"prompt_hash"`
	Kind       Kind   `json:"kind"`
	Complete   bool   `json:"complete"`
}

type OMPContextHashedReference struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}

type OMPContextHistoryReference struct {
	ID            string `json:"id"`
	SourceRef     string `json:"source_ref"`
	BodyHash      string `json:"body_hash"`
	TokenEstimate int    `json:"token_estimate"`
	Reason        string `json:"reason"`
}

type OMPContextPlanReference struct {
	SourceRef   string `json:"source_ref"`
	SourceHash  string `json:"source_hash"`
	Disposition string `json:"disposition,omitempty"`
}

type OMPContextShadowPlan struct {
	SchemaVersion      string
	ShadowOnly         bool
	ActiveMode         string
	CandidateMode      string
	PinnedReferences   []OMPContextPlanReference
	SelectedReferences []OMPContextPlanReference
}

type OMPContextHistoryRow struct {
	ID         string
	SourceRef  string
	Body       string
	Completed  bool
	Superseded bool
	Document   bool
	Unresolved bool
}

type OMPContextEphemeral struct {
	OriginalTask     string
	DecisionDelta    string
	FrozenFindingIDs []string
	OwnershipPaths   []string
	ForbiddenPaths   []string
}

type OMPContextBindingInput struct {
	WorkspaceID     string
	SpecID          string
	TaskID          string
	Phase           string
	SessionID       string
	ProviderLabel   string
	DeliveryOptions ContextDeliveryOptions
	Delivery        ContextDeliveryResult
	Ephemeral       OMPContextEphemeral
	History         []OMPContextHistoryRow
	ShadowPlan      *OMPContextShadowPlan
}

func (OMPContextBindingInput) MarshalJSON() ([]byte, error) {
	return nil, ErrOMPContextBodySerialization
}

func (OMPContextEphemeral) MarshalJSON() ([]byte, error) {
	return nil, ErrOMPContextBodySerialization
}

func (OMPContextHistoryRow) MarshalJSON() ([]byte, error) {
	return nil, ErrOMPContextBodySerialization
}

type OMPContextBindingReceipt struct {
	SchemaVersion         string                        `json:"schema_version"`
	Event                 string                        `json:"event"`
	WorkspaceID           string                        `json:"workspace_id"`
	SpecID                string                        `json:"spec_id"`
	TaskID                string                        `json:"task_id"`
	Phase                 string                        `json:"phase"`
	SessionID             string                        `json:"session_id"`
	OptionsHash           string                        `json:"options_hash"`
	BindingHash           string                        `json:"binding_hash"`
	SnapshotHash          string                        `json:"snapshot_hash"`
	PromptManifestHash    string                        `json:"prompt_manifest_hash"`
	FullDocumentRefs      []OMPContextDocumentReference `json:"full_document_refs"`
	RequiredEphemeralRefs []OMPContextHashedReference   `json:"required_ephemeral_refs"`
	EligibleHistoryRefs   []OMPContextHistoryReference  `json:"eligible_history_refs"`
	ShadowPlanRefs        []OMPContextPlanReference     `json:"shadow_plan_refs"`
}

type OMPContextTerminalReceipt struct {
	SchemaVersion         string                        `json:"schema_version"`
	Event                 string                        `json:"event"`
	BindingHash           string                        `json:"binding_hash"`
	ExactMatch            bool                          `json:"exact_match"`
	Admission             bool                          `json:"admission"`
	Reason                string                        `json:"reason"`
	SnapshotHash          string                        `json:"snapshot_hash"`
	PromptManifestHash    string                        `json:"prompt_manifest_hash"`
	FullDocumentRefs      []OMPContextDocumentReference `json:"full_document_refs"`
	RequiredEphemeralRefs []OMPContextHashedReference   `json:"required_ephemeral_refs"`
}

func OMPWorkerResultSchema() []string {
	return []string{"owned_paths", "changed_files", "verification", "blockers", "next_required_step"}
}
