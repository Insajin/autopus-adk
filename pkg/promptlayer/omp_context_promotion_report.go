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
	AutoVersion         string `json:"auto_version"`
	AutoBinarySHA256    string `json:"auto_binary_sha256"`
	OMPVersion          string `json:"omp_version"`
	OMPExecutableSHA256 string `json:"omp_executable_sha256"`
	ExecutionClass      string `json:"execution_class"`
	RuntimeKind         string `json:"runtime_kind"`
}

type OMPContextPromotionTaskV1 struct {
	TaskIDDigest string `json:"task_id_digest"`
	Order        string `json:"order"`
}

type OMPContextPromotionObservationV1 struct {
	Sequence          int    `json:"sequence"`
	TaskIDDigest      string `json:"task_id_digest"`
	Variant           string `json:"variant"`
	Provider          string `json:"provider"`
	ModelScopeDigest  string `json:"model_scope_digest"`
	EndpointClass     string `json:"endpoint_class"`
	Transport         string `json:"transport"`
	CredentialMode    string `json:"credential_mode"`
	ExecutionMode     string `json:"execution_mode"`
	StartedAt         string `json:"started_at"`
	CompletedAt       string `json:"completed_at"`
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	TotalTokens       int64  `json:"total_tokens"`
	ObservationDigest string `json:"observation_digest"`
	UsageDigest       string `json:"usage_digest"`
	IntegrityPassed   bool   `json:"integrity_passed"`
	SecurityPassed    bool   `json:"security_passed"`
	QualityScore      int64  `json:"quality_score"`
	FallbackVerified  bool   `json:"fallback_verified"`
	RollbackVerified  bool   `json:"rollback_verified"`
	CleanupVerified   bool   `json:"cleanup_verified"`
	RetryCount        int    `json:"retry_count"`
	MaxConcurrency    int    `json:"max_concurrency"`
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
	Provider             string                             `json:"provider"`
	ModelScopeDigest     string                             `json:"model_scope_digest"`
	CohortManifestDigest string                             `json:"cohort_manifest_digest"`
	OrderSeed            string                             `json:"order_seed"`
	OraclePolicyDigest   string                             `json:"oracle_policy_digest"`
	Tasks                []OMPContextPromotionTaskV1        `json:"tasks"`
	Observations         []OMPContextPromotionObservationV1 `json:"observations"`
	Gates                []OMPContextPromotionGateResultV1  `json:"gates"`
}
