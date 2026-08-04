package cli

const (
	workflowContextObserveCallRequestSchema = "autopus.omp_context_observe_call_request.v1"
	workflowContextObserveCallResultSchema  = "autopus.omp_context_observe_call_result.v1"
)

type workflowContextObserveCallRequest struct {
	SchemaVersion string `json:"schema_version"`
	Sequence      int    `json:"sequence"`
	PairSequence  int    `json:"pair_sequence"`
	TaskID        string `json:"task_id"`
	Prompt        string `json:"prompt"`
	Variant       string `json:"variant"`
}

type workflowContextObserveCallTokenUsage struct {
	PrimaryInputTokens      int64 `json:"primary_input_tokens"`
	PrimaryOutputTokens     int64 `json:"primary_output_tokens"`
	MaintenanceInputTokens  int64 `json:"maintenance_input_tokens"`
	MaintenanceOutputTokens int64 `json:"maintenance_output_tokens"`
	TotalTokens             int64 `json:"total_tokens"`
}

type workflowContextObserveCallLifecycleFacts struct {
	PreCompactionEvents  int  `json:"pre_compaction_events"`
	PostCompactionEvents int  `json:"post_compaction_events"`
	NativeStarts         int  `json:"native_compaction_starts"`
	NativeEnds           int  `json:"native_compaction_ends"`
	ProviderTurns        int  `json:"provider_turns"`
	SameProcess          bool `json:"same_process"`
	SameSession          bool `json:"same_session"`
	TerminalIdle         bool `json:"terminal_idle"`
	Sandboxed            bool `json:"sandboxed"`
}

type workflowContextObserveCallCleanupFacts struct {
	OwnedRootsCreated int `json:"owned_roots_created"`
	OwnedRootsRemoved int `json:"owned_roots_removed"`
	OwnedRootsRemain  int `json:"owned_roots_remaining"`
	ProcessesRemain   int `json:"processes_remaining"`
}

type workflowContextObserveCallResult struct {
	SchemaVersion            string                                   `json:"schema_version"`
	Sequence                 int                                      `json:"sequence"`
	PairSequence             int                                      `json:"pair_sequence"`
	TaskID                   string                                   `json:"task_id"`
	Variant                  string                                   `json:"variant"`
	Provider                 string                                   `json:"provider"`
	Model                    string                                   `json:"model"`
	ExecutionClass           string                                   `json:"execution_class"`
	ProductionPathEquivalent bool                                     `json:"production_path_equivalent"`
	StartedAt                string                                   `json:"started_at"`
	CompletedAt              string                                   `json:"completed_at"`
	AssistantText            string                                   `json:"assistant_text"`
	TokenUsage               workflowContextObserveCallTokenUsage     `json:"token_usage"`
	LifecycleFacts           workflowContextObserveCallLifecycleFacts `json:"lifecycle_facts"`
	CleanupFacts             workflowContextObserveCallCleanupFacts   `json:"cleanup_facts"`
}

type workflowContextObserveCallOptions struct {
	ProjectDir        string
	SpecID            string
	Provider          string
	Model             string
	Endpoint          string
	CredentialLocator string
	Executable        string
}
