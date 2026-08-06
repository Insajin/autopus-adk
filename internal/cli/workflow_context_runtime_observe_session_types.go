package cli

import (
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

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
	EvidenceID               string                              `json:"evidence_id,omitempty"`
	ReportDigest             string                              `json:"report_digest,omitempty"`
	TaskIDDigest             string                              `json:"task_id_digest,omitempty"`
	Variant                  string                              `json:"variant,omitempty"`
	AssistantText            string                              `json:"assistant_text,omitempty"`
	OutputDigest             string                              `json:"output_digest,omitempty"`
	SessionDigest            string                              `json:"session_digest,omitempty"`
	ProviderAuthorityDigest  string                              `json:"provider_authority_digest,omitempty"`
	ProcessReused            bool                                `json:"process_reused,omitempty"`
	CompactionCycles         int                                 `json:"compaction_cycles,omitempty"`
	PreCompactionACKs        int                                 `json:"pre_compaction_acks,omitempty"`
	PostCompactionACKs       int                                 `json:"post_compaction_acks,omitempty"`
	CanonicalReadmissions    int                                 `json:"canonical_readmissions,omitempty"`
	EphemeralReadmissions    int                                 `json:"ephemeral_readmissions,omitempty"`
	Usage                    *workflowContextObserveSessionUsage `json:"usage,omitempty"`
	CallsCompleted           int                                 `json:"calls_completed,omitempty"`
	OwnedRootsRemaining      int                                 `json:"owned_roots_remaining,omitempty"`
	CleanupVerified          bool                                `json:"cleanup_verified,omitempty"`
	ProcessesRemaining       int                                 `json:"processes_remaining,omitempty"`
	ErrorCode                string                              `json:"error_code,omitempty"`
}

type workflowContextObserveSessionOptions struct {
	ProjectDir          string
	SpecID              string
	Provider            string
	Model               string
	ModelContextWindow  int
	Endpoint            string
	CredentialLocator   string
	Executable          string
	TargetGitCommit     string
	SandboxMode         pipelineOMPActiveSandboxMode
	WorkspaceID         string
	ProducerRepository  string
	ProducerWorkflowRef string
	ProducerRunID       string
	ProducerRunAttempt  int
	CandidateRepository string
	PolicyID            string
	OraclePolicyDigest  string
	PromotionPolicy     promptlayer.OMPContextPromotionPolicyV1
	EvidenceValidFor    time.Duration
}

func workflowContextObserveSessionErrorCode(runErr error) string {
	if runErr == nil {
		return ""
	}
	message := strings.ToLower(runErr.Error())
	switch {
	case strings.Contains(message, "handshake is invalid"),
		strings.Contains(message, "shutdown is invalid"),
		strings.Contains(message, "input continued after shutdown"),
		strings.Contains(message, "call ") &&
			(containsAny(message, " is invalid", "repeats a task", "pair authority changed")):
		return "input_invalid"
	case containsAny(message,
		"private or unsafe output",
		"quality oracle changed",
		"process lifecycle changed",
		"persistent process pair is invalid",
		"provider authority binding is unstable",
		"task or reusable-session cardinality is invalid"):
		return "runtime_invariant_failed"
	case strings.Contains(message, "failed readback"):
		return "runtime_readback_failed"
	case strings.Contains(message, "canonical admission failed"):
		return "runtime_admission_failed"
	}
	class, _ := classifyOperationalError("", runErr)
	if class == "unknown" {
		return "runtime_failed"
	}
	return class
}
