package qualityloop

const (
	SchemaVersion = "improvement_candidate.v1"

	TaxonomyPromptContractGap      = "prompt_contract_gap"
	TaxonomyModelRoutingGap        = "model_routing_gap"
	TaxonomySkillOrPlaybookGap     = "skill_or_playbook_gap"
	TaxonomySourceSetupGap         = "source_setup_gap"
	TaxonomyProductBug             = "product_bug"
	TaxonomyEvaluatorOrOracleGap   = "evaluator_or_oracle_gap"
	TaxonomySafetyPolicyGap        = "safety_policy_gap"
	TaxonomyStaleOrMissingEvidence = "stale_or_missing_evidence"
	TaxonomyUnsafeMutationBoundary = "unsafe_mutation_boundary"
	TaxonomyUserExpectationGap     = "user_expectation_gap"

	KindQAMESHRepairHandoff      = "qamesh_repair_handoff"
	KindSkillEvolveCandidate     = "skill_evolve_candidate"
	KindPromptLayerUpdate        = "prompt_layer_update"
	KindModelRoutingPolicy       = "model_routing_policy"
	KindPlaybookCandidate        = "playbook_candidate"
	KindSourceSetupMission       = "source_setup_mission"
	KindWorkspaceEvolutionSignal = "workspace_evolution_signal"
	KindOperatingPackCandidate   = "operating_pack_candidate"
	KindProductBugFix            = "product_bug_fix"
	KindImplementationSpec       = "implementation_spec_candidate"
	KindSafetyPolicyPatch        = "safety_policy_patch"
	KindEvalCalibrationTask      = "eval_calibration_task"
	KindLaunchGateBlocker        = "launch_gate_blocker"
	KindAgentEvalRemediation     = "agent_eval_remediation"
	KindEvidenceDiscipline       = "evidence_discipline_candidate"
	KindUserExpectationCandidate = "user_expectation_candidate"

	StatusObserved         = "observed"
	StatusNormalized       = "normalized"
	StatusRouted           = "routed"
	StatusQuarantined      = "quarantined"
	StatusBlocked          = "blocked"
	StatusAwaitingReplay   = "awaiting_replay"
	StatusReplayFailed     = "replay_failed"
	StatusReplayPassed     = "replay_passed"
	StatusApprovalRequired = "approval_required"
	StatusApproved         = "approved"
	StatusApplied          = "applied"
	StatusVerified         = "verified"
	StatusArchived         = "archived"
	StatusRejected         = "rejected"

	BandHigh   = "high"
	BandMedium = "medium"
	BandLow    = "low"

	MethodDeterministicOracle = "deterministic_oracle"
	MethodContractMapping     = "contract_mapping"
	MethodMixedEvidence       = "mixed_evidence"
	MethodLLMAssistedReview   = "llm_assisted_review"
	MethodHumanReviewed       = "human_reviewed"

	EvidenceDeterministic = "deterministic"
	EvidenceMixed         = "mixed"
	EvidenceLLMOnly       = "llm_only"
	EvidenceConflicting   = "conflicting"
	EvidenceMissing       = "missing"

	PolicyDisabled            = "disabled"
	PolicyAdvisoryOnly        = "advisory_only"
	PolicyHumanReviewRequired = "human_review_required"
	PolicyReplayRequired      = "replay_required"
	PolicyApprovalRequired    = "approval_required"
	PolicyReadyForApply       = "ready_for_apply"
	PolicyNoAction            = "no_action"

	RedactionRedacted     = "redacted"
	RedactionMetadataOnly = "metadata_only"
	RetentionAudit        = "improvement_candidate_audit"
)

type FailureInput struct {
	SourceArtifactType         string
	SourceID                   string
	WorkspaceID                string
	OwningRepo                 string
	Owner                      string
	FailureFingerprint         string
	FailureTaxonomy            string
	ReasonCode                 string
	ReasonCodes                []string
	DeterministicEvidence      bool
	DeterministicProductDefect bool
	ConflictingLLMNarrative    bool
	EvidenceStrength           string
	EvidenceGapRefs            []string
	EvidenceRefs               []string
	SourceRefs                 []string
	SourceHashes               []string
	AffectedRefs               []string
	AffectedAcceptanceIDs      []string
	TargetArtifact             string
	OwnedPaths                 []string
	DoNotModifyPaths           []string
	QAMESHStatus               string
	RedactionStatus            string
	ReplayFreshness            string
	ConfidenceOverride         float64
	ProposedActionKind         string
	UntrustedSummary           string
	ExpectedValidation         string
	RollbackPath               string
	RawPayloadPresent          bool
	ProviderWriteCallCount     int
}

type NormalizeResult struct {
	Candidates  []ImprovementCandidate `json:"candidates"`
	Unsupported []UnsupportedFailure   `json:"unsupported,omitempty"`
}

type UnsupportedFailure struct {
	SourceID string `json:"source_id"`
	Reason   string `json:"reason"`
}

type ImprovementCandidate struct {
	SchemaVersion               string   `json:"schema_version"`
	CandidateID                 string   `json:"candidate_id"`
	CandidateKind               string   `json:"candidate_kind"`
	Status                      string   `json:"status"`
	Active                      bool     `json:"active"`
	WorkspaceID                 string   `json:"workspace_id,omitempty"`
	OwningRepo                  string   `json:"owning_repo,omitempty"`
	Owner                       string   `json:"owner,omitempty"`
	FailureFingerprint          string   `json:"failure_fingerprint"`
	FailureTaxonomy             string   `json:"failure_taxonomy"`
	ReasonCodes                 []string `json:"reason_codes"`
	ClassificationConfidence    float64  `json:"classification_confidence"`
	ConfidenceBand              string   `json:"confidence_band"`
	ClassificationMethod        string   `json:"classification_method"`
	EvidenceStrength            string   `json:"evidence_strength"`
	EvidenceGapRefs             []string `json:"evidence_gap_refs,omitempty"`
	LowConfidenceReviewRequired bool     `json:"low_confidence_review_required"`
	Severity                    string   `json:"severity,omitempty"`
	DeterministicAuthority      bool     `json:"deterministic_authority"`
	SourceFailureRefs           []string `json:"source_failure_refs,omitempty"`
	SourceArtifactType          string   `json:"source_artifact_type,omitempty"`
	SourceHashes                []string `json:"source_hashes,omitempty"`
	EvidenceRefs                []string `json:"evidence_refs,omitempty"`
	SourceRefs                  []string `json:"source_refs,omitempty"`
	ForbiddenWriteSurfaces      []string `json:"forbidden_write_surfaces,omitempty"`
	QualityIndexRefs            []string `json:"quality_index_refs,omitempty"`
	AffectedOutputs             []string `json:"affected_outputs,omitempty"`
	AffectedRefs                []string `json:"affected_refs,omitempty"`
	AffectedAcceptanceIDs       []string `json:"affected_acceptance_ids,omitempty"`
	RecommendedRoute            string   `json:"recommended_route,omitempty"`
	RouteTargets                []string `json:"route_targets,omitempty"`
	TargetArtifact              string   `json:"target_artifact,omitempty"`
	SourceOwnedTargetPath       string   `json:"source_owned_target_path,omitempty"`
	GeneratedSurfaceValidation  string   `json:"generated_surface_validation,omitempty"`
	Risk                        string   `json:"risk,omitempty"`
	ExpectedValidation          string   `json:"expected_validation,omitempty"`
	RollbackPath                string   `json:"rollback_path,omitempty"`
	ProposedAction              string   `json:"proposed_action,omitempty"`
	ProposedDigest              string   `json:"proposed_digest,omitempty"`
	GenerationPromptDigest      string   `json:"generation_prompt_digest,omitempty"`
	RepairActionPolicy          string   `json:"repair_action_policy"`
	RepairActionEnabled         bool     `json:"repair_action_enabled"`
	ApplyEnabled                bool     `json:"apply_enabled"`
	RouteMetadata               []string `json:"route_metadata,omitempty"`
	Verified                    bool     `json:"verified"`
	AttemptCount                int      `json:"attempt_count"`
	ReplayAttemptCount          int      `json:"replay_attempt_count"`
	MaxReplayAttempts           int      `json:"max_replay_attempts"`
	LastReplayStatus            string   `json:"last_replay_status,omitempty"`
	NonConvergenceReason        string   `json:"non_convergence_reason,omitempty"`
	SupersedesCandidateID       string   `json:"supersedes_candidate_id,omitempty"`
	RelatedCandidateIDs         []string `json:"related_candidate_ids,omitempty"`
	ReplayPlan                  string   `json:"replay_plan,omitempty"`
	ReplayEvidenceRefs          []string `json:"replay_evidence_refs,omitempty"`
	ApprovalGate                string   `json:"approval_gate,omitempty"`
	ApprovalRefs                []string `json:"approval_refs,omitempty"`
	SafetyGate                  string   `json:"safety_gate,omitempty"`
	SafetyReasonCodes           []string `json:"safety_reason_codes,omitempty"`
	ProviderWriteCallCount      int      `json:"provider_write_call_count"`
	RedactionStatus             string   `json:"redaction_status"`
	RetentionClass              string   `json:"retention_class"`
	RawPayloadPresent           bool     `json:"raw_payload_present"`
	AuditRefs                   []string `json:"audit_refs"`
}

type CandidateDraft struct {
	WorkspaceID            string
	EvidenceRefs           []string
	DisplayRefs            []string
	TargetArtifact         string
	RawPayloadPresent      bool
	ProviderWriteCallCount int
}

type SafetyDecision struct {
	Accepted               bool
	Status                 string
	ReasonCodes            []string
	ProviderWriteCallCount int
	Active                 bool
	RawRetainedPayload     string
}

type LifecycleSignal struct {
	ReplayStatus           string
	ReplayEvidenceRefs     []string
	RequiresApproval       bool
	HumanApprovalRefs      []string
	ApplyCompleted         bool
	OriginalBlockerCleared bool
	PostApplyEvidenceRefs  []string
	ReplayRunIndexMissing  bool
	ReplayOutsideProject   bool
	ReplayNonDeterministic bool
	ReplayMissingACMapping bool
	ReplayFreshness        string
}
