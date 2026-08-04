package cli

type pipelineOMPContextObservationCandidate struct {
	Repository     string `json:"repository"`
	Revision       string `json:"revision"`
	TreeSHA        string `json:"tree_sha"`
	ArtifactSHA256 string `json:"artifact_sha256"`
}

type pipelineOMPContextObservationPolicy struct {
	PolicyID                string `json:"policy_id"`
	PolicyDigest            string `json:"policy_digest"`
	HistoryMode             string `json:"history_mode"`
	MemoryMode              string `json:"memory_mode"`
	MinPairCount            int    `json:"min_pair_count"`
	MinReductionBasisPoints int64  `json:"min_reduction_basis_points"`
}

type pipelineOMPContextObservationRuntime struct {
	OMPExecutableSHA256          string `json:"omp_executable_sha256"`
	OMPVersion                   string `json:"omp_version"`
	AutoBinarySHA256             string `json:"auto_binary_sha256"`
	AutoVersion                  string `json:"auto_version"`
	ExecutionClass               string `json:"execution_class"`
	RuntimeKind                  string `json:"runtime_kind"`
	ProductionPathEquivalent     bool   `json:"production_path_equivalent"`
	PipelineImplementationDigest string `json:"pipeline_implementation_digest"`
}

type pipelineOMPContextObservationTask struct {
	TaskIDDigest string `json:"task_id_digest"`
	Order        string `json:"order"`
}

type pipelineOMPContextTokenUsage struct {
	PrimaryInputTokens      int64 `json:"primary_input_tokens"`
	PrimaryOutputTokens     int64 `json:"primary_output_tokens"`
	MaintenanceInputTokens  int64 `json:"maintenance_input_tokens"`
	MaintenanceOutputTokens int64 `json:"maintenance_output_tokens"`
	TotalTokens             int64 `json:"total_tokens"`
}

type pipelineOMPContextIntegrityFacts struct {
	RequiredDocumentRefs    int `json:"required_document_refs"`
	MatchedDocumentRefs     int `json:"matched_document_refs"`
	RequiredEphemeralFields int `json:"required_ephemeral_fields"`
	MatchedEphemeralFields  int `json:"matched_ephemeral_fields"`
	HashMismatches          int `json:"hash_mismatches"`
	DocumentOmissions       int `json:"document_omissions"`
	MemoryInjections        int `json:"memory_injections"`
	SetupProviderRequests   int `json:"setup_provider_requests"`
	PrimaryProviderRequests int `json:"primary_provider_requests"`
	CompactionProviderCalls int `json:"compaction_provider_requests"`
	TotalProviderRequests   int `json:"total_external_provider_requests"`
	UnexpectedProviderCalls int `json:"unexpected_provider_requests"`
}

type pipelineOMPContextSecurityFacts struct {
	AuthorizedGatewayRequests  int `json:"authorized_gateway_requests"`
	UnexpectedLoopbackRequests int `json:"unexpected_loopback_requests"`
	ChildCredentialExposures   int `json:"child_upstream_credential_exposures"`
	ChildEndpointExposures     int `json:"child_upstream_endpoint_exposures"`
	RawBodyPersistedFields     int `json:"raw_body_persisted_fields"`
	SecretMatches              int `json:"secret_matches"`
	AbsolutePathLeaks          int `json:"absolute_path_leaks"`
	PromptInjectionExecutions  int `json:"prompt_injection_executions"`
	SecurityFindings           int `json:"security_findings"`
}

type pipelineOMPContextQualityFacts struct {
	Assertions       int `json:"assertions"`
	PassedAssertions int `json:"passed_assertions"`
}

type pipelineOMPContextFallbackFacts struct {
	Trials                              int `json:"trials"`
	CanonicalFullRestarts               int `json:"canonical_full_restarts"`
	ExactMatches                        int `json:"exact_matches"`
	OptimizedProviderCallsAfterMismatch int `json:"optimized_provider_calls_after_mismatch"`
}

type pipelineOMPContextRollbackFacts struct {
	Trials                     int `json:"trials"`
	EffectiveShadowReadbacks   int `json:"effective_shadow_readbacks"`
	ReadbackMismatches         int `json:"readback_mismatches"`
	InheritedOptimizedSessions int `json:"inherited_optimized_sessions"`
}

type pipelineOMPContextCleanupFacts struct {
	OwnedRootsCreated int `json:"owned_roots_created"`
	OwnedRootsRemoved int `json:"owned_roots_removed"`
	OwnedRootsRemain  int `json:"owned_roots_remaining"`
	UserRootsAccessed int `json:"user_roots_accessed"`
	PathEscapeEvents  int `json:"path_escape_events"`
	ModeViolations    int `json:"mode_violations"`
}

type pipelineOMPContextObservationCall struct {
	Sequence         int                              `json:"sequence"`
	PairSequence     int                              `json:"pair_sequence"`
	RetryCount       int                              `json:"retry_count"`
	MaxConcurrency   int                              `json:"max_concurrency"`
	TaskIDDigest     string                           `json:"task_id_digest"`
	Variant          string                           `json:"variant"`
	Provider         string                           `json:"provider"`
	Model            string                           `json:"model"`
	ModelScopeDigest string                           `json:"model_scope_digest"`
	EndpointClass    string                           `json:"endpoint_class"`
	Transport        string                           `json:"transport"`
	CredentialMode   string                           `json:"credential_mode"`
	ExecutionMode    string                           `json:"execution_mode"`
	StartedAt        string                           `json:"started_at"`
	CompletedAt      string                           `json:"completed_at"`
	TokenUsage       pipelineOMPContextTokenUsage     `json:"token_usage"`
	IntegrityFacts   pipelineOMPContextIntegrityFacts `json:"integrity_facts"`
	SecurityFacts    pipelineOMPContextSecurityFacts  `json:"security_facts"`
	QualityFacts     pipelineOMPContextQualityFacts   `json:"quality_facts"`
	FallbackFacts    pipelineOMPContextFallbackFacts  `json:"fallback_facts"`
	RollbackFacts    pipelineOMPContextRollbackFacts  `json:"rollback_facts"`
	CleanupFacts     pipelineOMPContextCleanupFacts   `json:"cleanup_facts"`
	TaskDigest       string                           `json:"task_digest"`
	OutputDigest     string                           `json:"output_digest"`
	OracleDigest     string                           `json:"oracle_digest"`
}

type pipelineOMPContextObservationV1 struct {
	SchemaVersion       string                                 `json:"schema_version"`
	EvidenceID          string                                 `json:"evidence_id"`
	ChallengeDigest     string                                 `json:"challenge_digest"`
	RunID               string                                 `json:"run_id"`
	Attempt             int                                    `json:"attempt"`
	TrustLane           string                                 `json:"trust_lane"`
	ProducerRepository  string                                 `json:"producer_repository"`
	ProducerWorkflowRef string                                 `json:"producer_workflow_ref"`
	Candidate           pipelineOMPContextObservationCandidate `json:"candidate"`
	Policy              pipelineOMPContextObservationPolicy    `json:"policy"`
	Runtime             pipelineOMPContextObservationRuntime   `json:"runtime"`
	Provider            string                                 `json:"provider"`
	Model               string                                 `json:"model"`
	ModelScopeDigest    string                                 `json:"model_scope_digest"`
	CredentialLocator   string                                 `json:"credential_locator"`
	CohortDigest        string                                 `json:"cohort_manifest_digest"`
	OrderSeed           string                                 `json:"order_seed"`
	OraclePolicyDigest  string                                 `json:"oracle_policy_digest"`
	Tasks               []pipelineOMPContextObservationTask    `json:"tasks"`
	Calls               []pipelineOMPContextObservationCall    `json:"calls"`
}
