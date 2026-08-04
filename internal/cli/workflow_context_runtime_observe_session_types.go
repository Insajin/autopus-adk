package cli

const (
	workflowContextObserveSessionCommandSchema  = "autopus.omp_context_observe_session_command.v1"
	workflowContextObserveSessionResponseSchema = "autopus.omp_context_observe_session_response.v1"
)

type workflowContextObserveSessionCommand struct {
	SchemaVersion   string `json:"schema_version"`
	Type            string `json:"type"`
	ChallengeDigest string `json:"challenge_digest,omitempty"`
	Sequence        int    `json:"sequence,omitempty"`
	PairSequence    int    `json:"pair_sequence,omitempty"`
	TaskIDDigest    string `json:"task_id_digest,omitempty"`
	Variant         string `json:"variant,omitempty"`
	Prompt          string `json:"prompt,omitempty"`
}

type workflowContextObserveSessionUsage struct {
	PrimaryInputTokens      int64 `json:"primary_input_tokens"`
	PrimaryOutputTokens     int64 `json:"primary_output_tokens"`
	MaintenanceInputTokens  int64 `json:"maintenance_input_tokens"`
	MaintenanceOutputTokens int64 `json:"maintenance_output_tokens"`
	TotalTokens             int64 `json:"total_tokens"`
}

type workflowContextObserveSessionResponse struct {
	SchemaVersion            string                              `json:"schema_version"`
	Type                     string                              `json:"type"`
	ExecutionClass           string                              `json:"execution_class"`
	RuntimeKind              string                              `json:"runtime_kind"`
	ProductionPathEquivalent bool                                `json:"production_path_equivalent"`
	ImplementationDigest     string                              `json:"implementation_digest,omitempty"`
	ModelScopeDigest         string                              `json:"model_scope_digest,omitempty"`
	SourceCommit             string                              `json:"source_commit,omitempty"`
	SourceTree               string                              `json:"source_tree,omitempty"`
	OMPVersion               string                              `json:"omp_version,omitempty"`
	OMPExecutableSHA256      string                              `json:"omp_executable_sha256,omitempty"`
	Sequence                 int                                 `json:"sequence,omitempty"`
	PairSequence             int                                 `json:"pair_sequence,omitempty"`
	TaskIDDigest             string                              `json:"task_id_digest,omitempty"`
	Variant                  string                              `json:"variant,omitempty"`
	AssistantText            string                              `json:"assistant_text,omitempty"`
	OutputDigest             string                              `json:"output_digest,omitempty"`
	SessionDigest            string                              `json:"session_digest,omitempty"`
	ProcessReused            bool                                `json:"process_reused,omitempty"`
	CompactionCycles         int                                 `json:"compaction_cycles,omitempty"`
	Usage                    *workflowContextObserveSessionUsage `json:"usage,omitempty"`
	CallsCompleted           int                                 `json:"calls_completed,omitempty"`
	OwnedRootsRemaining      int                                 `json:"owned_roots_remaining,omitempty"`
	ProcessesRemaining       int                                 `json:"processes_remaining,omitempty"`
}

type workflowContextObserveSessionOptions struct {
	ProjectDir        string
	SpecID            string
	Provider          string
	Model             string
	Endpoint          string
	CredentialLocator string
	Executable        string
	TargetGitCommit   string
}
