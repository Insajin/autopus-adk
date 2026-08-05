package promptlayer

const OMPContextPromotionReportSchemaV1 = "autopus.omp_context_promotion_report.v1"

type OMPContextPromotionProducerV1 struct {
	Repository  string `json:"repository"`
	WorkflowRef string `json:"workflow_ref"`
	RunID       string `json:"run_id"`
	RunAttempt  int    `json:"run_attempt"`
}

type OMPContextPromotionCandidateV1 struct {
	Repository     string `json:"repository"`
	Revision       string `json:"revision"`
	TreeSHA        string `json:"tree_sha"`
	ArtifactSHA256 string `json:"artifact_sha256"`
}

type OMPContextPromotionPolicyReportV1 struct {
	PolicyID                string `json:"policy_id"`
	PolicyDigest            string `json:"policy_digest"`
	HistoryMode             string `json:"history_mode"`
	MemoryMode              string `json:"memory_mode"`
	MinPairCount            int    `json:"min_pair_count"`
	MinReductionBasisPoints int64  `json:"min_reduction_basis_points"`
}

type OMPContextPromotionRuntimeV1 struct {
	AutoVersion                  string `json:"auto_version"`
	AutoBinarySHA256             string `json:"auto_binary_sha256"`
	OMPVersion                   string `json:"omp_version"`
	OMPExecutableSHA256          string `json:"omp_executable_sha256"`
	ExecutionClass               string `json:"execution_class"`
	ProductionPathEquivalent     bool   `json:"production_path_equivalent"`
	RuntimeKind                  string `json:"runtime_kind"`
	PipelineImplementationDigest string `json:"pipeline_implementation_digest"`
}

type OMPContextPromotionSessionFactsV1 struct {
	FullProcessStarts         int `json:"full_process_starts"`
	OptimizedProcessStarts    int `json:"optimized_process_starts"`
	FullSessionCount          int `json:"full_session_count"`
	OptimizedSessionCount     int `json:"optimized_session_count"`
	MaxConcurrency            int `json:"max_concurrency"`
	CrossSessionContamination int `json:"cross_session_contamination"`
}

type OMPContextPromotionTaskV1 struct {
	TaskIDDigest string `json:"task_id_digest"`
	Order        string `json:"order"`
}

type OMPContextPromotionObservationV1 struct {
	Sequence                   int    `json:"sequence"`
	TaskIDDigest               string `json:"task_id_digest"`
	Variant                    string `json:"variant"`
	SessionReceiptDigest       string `json:"session_receipt_digest"`
	SessionSequence            int    `json:"session_sequence"`
	ProcessReused              bool   `json:"process_reused"`
	Provider                   string `json:"provider"`
	ModelScopeDigest           string `json:"model_scope_digest"`
	EndpointClass              string `json:"endpoint_class"`
	Transport                  string `json:"transport"`
	CredentialMode             string `json:"credential_mode"`
	ProviderAuthorityDigest    string `json:"provider_authority_digest"`
	ExecutionMode              string `json:"execution_mode"`
	StartedAt                  string `json:"started_at"`
	CompletedAt                string `json:"completed_at"`
	InputTokens                int64  `json:"input_tokens"`
	OutputTokens               int64  `json:"output_tokens"`
	TotalTokens                int64  `json:"total_tokens"`
	SetupProviderRequests      int    `json:"setup_provider_requests"`
	CompactionProviderRequests int    `json:"compaction_provider_requests"`
	PrimaryProviderRequests    int    `json:"primary_provider_requests"`
	PreCompactionACKs          int    `json:"pre_compaction_acks"`
	PostCompactionACKs         int    `json:"post_compaction_acks"`
	CanonicalReadmissions      int    `json:"canonical_readmissions"`
	EphemeralReadmissions      int    `json:"ephemeral_readmissions"`
	TotalProviderRequests      int    `json:"total_provider_requests"`
	ObservationDigest          string `json:"observation_digest"`
	UsageDigest                string `json:"usage_digest"`
	IntegrityPassed            bool   `json:"integrity_passed"`
	SecurityPassed             bool   `json:"security_passed"`
	QualityScore               int64  `json:"quality_score"`
	FallbackVerified           bool   `json:"fallback_verified"`
	RollbackVerified           bool   `json:"rollback_verified"`
	CleanupVerified            bool   `json:"cleanup_verified"`
	RetryCount                 int    `json:"retry_count"`
	MaxConcurrency             int    `json:"max_concurrency"`
}

type OMPContextPromotionGateResultV1 struct {
	GateID        string `json:"gate_id"`
	Status        string `json:"status"`
	ObservedValue string `json:"observed_value"`
	RequiredValue string `json:"required_value"`
	Reason        string `json:"reason"`
}

type OMPContextPromotionReportV1 struct {
	SchemaVersion        string                             `json:"schema_version"`
	EvidenceID           string                             `json:"evidence_id"`
	ChallengeDigest      string                             `json:"challenge_digest"`
	TrustLane            string                             `json:"trust_lane"`
	Producer             OMPContextPromotionProducerV1      `json:"producer"`
	Candidate            OMPContextPromotionCandidateV1     `json:"candidate"`
	Policy               OMPContextPromotionPolicyReportV1  `json:"policy"`
	Runtime              OMPContextPromotionRuntimeV1       `json:"runtime"`
	SessionFacts         OMPContextPromotionSessionFactsV1  `json:"session_facts"`
	Provider             string                             `json:"provider"`
	ModelScopeDigest     string                             `json:"model_scope_digest"`
	CohortManifestDigest string                             `json:"cohort_manifest_digest"`
	OrderSeed            string                             `json:"order_seed"`
	OraclePolicyDigest   string                             `json:"oracle_policy_digest"`
	Tasks                []OMPContextPromotionTaskV1        `json:"tasks"`
	Observations         []OMPContextPromotionObservationV1 `json:"observations"`
	Gates                []OMPContextPromotionGateResultV1  `json:"gates"`
}
