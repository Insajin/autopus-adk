package skillevolve

type CandidateGenerationOptions struct {
	ProjectDir       string
	QualityIndexPath string
	QuarantineDir    string
	MinCount         int
	Creator          string
}

type CandidateGenerationResult struct {
	Candidates []CandidateBundle `json:"candidates"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: persisted candidate bundle JSON is the quarantine, replay, promotion, and archive contract.
// @AX:REASON: CLI output, on-disk bundle files, replay evidence updates, and promotion safety gates all share these fields.
type CandidateBundle struct {
	ID                        string              `json:"id"`
	Fingerprint               string              `json:"fingerprint,omitempty"`
	Status                    string              `json:"status"`
	Active                    bool                `json:"active"`
	Creator                   string              `json:"creator,omitempty"`
	RedactionStatus           string              `json:"redaction_status,omitempty"`
	SourceFailures            []SourceFailure     `json:"source_failures,omitempty"`
	SourceHashes              []string            `json:"source_hashes,omitempty"`
	AffectedRefs              []string            `json:"affected_refs,omitempty"`
	AffectedAcceptanceIDs     []string            `json:"affected_acceptance_ids,omitempty"`
	ProposedDigest            string              `json:"proposed_digest,omitempty"`
	GenerationPromptDigest    string              `json:"generation_prompt_digest,omitempty"`
	ReplayPlan                ReplayPlan          `json:"replay_plan,omitempty"`
	BundlePath                string              `json:"bundle_path,omitempty"`
	OwnedPaths                []string            `json:"owned_paths,omitempty"`
	ProposedFiles             []ProposedFile      `json:"proposed_files,omitempty"`
	LLMScore                  LLMScore            `json:"llm_score,omitempty"`
	PromotionReady            bool                `json:"promotion_ready,omitempty"`
	SafetyReasonCodes         []string            `json:"safety_reason_codes,omitempty"`
	Provenance                CandidateProvenance `json:"provenance,omitempty"`
	ReplayEvidenceRefs        []string            `json:"replay_evidence_refs,omitempty"`
	AffectedGeneratedSurfaces []string            `json:"affected_generated_surfaces,omitempty"`
}

type SourceFailure struct {
	Ref         string `json:"ref"`
	Hash        string `json:"hash"`
	EvidenceRef string `json:"evidence_ref"`
}

type CandidateProvenance struct {
	SourceFailureRefs         []string `json:"source_failure_refs,omitempty"`
	SourceHashes              []string `json:"source_hashes,omitempty"`
	EvidenceRefs              []string `json:"evidence_refs,omitempty"`
	GenerationPromptDigest    string   `json:"generation_prompt_digest,omitempty"`
	RedactionStatus           string   `json:"redaction_status,omitempty"`
	Creator                   string   `json:"creator,omitempty"`
	AffectedAcceptanceIDs     []string `json:"affected_acceptance_ids,omitempty"`
	AffectedSourceOfTruths    []string `json:"affected_source_of_truths,omitempty"`
	AffectedGeneratedSurfaces []string `json:"affected_generated_surfaces,omitempty"`
}

type ProposedFile struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type ReplayPlan struct {
	RunIndexPath   string           `json:"run_index_path,omitempty"`
	Commands       []ReplayCommand  `json:"commands,omitempty"`
	MustChecks     []ReplayCheckRef `json:"must_checks,omitempty"`
	AcceptanceRefs []string         `json:"acceptance_refs,omitempty"`
}

type ReplayCommand struct {
	Command string `json:"command"`
}

type ReplayCheckRef struct {
	ID            string `json:"id"`
	AcceptanceRef string `json:"acceptance_ref,omitempty"`
	Source        string `json:"source,omitempty"`
}

type LLMScore struct {
	Score     float64 `json:"score,omitempty"`
	Advisory  bool    `json:"advisory"`
	Authority string  `json:"authority,omitempty"`
}

type SafetyOptions struct {
	MaxCandidateBytes int
	OwnedPaths        []string
}

type SafetyResult struct {
	Allowed          bool     `json:"allowed"`
	ReplayAllowed    bool     `json:"replay_allowed"`
	PromotionAllowed bool     `json:"promotion_allowed"`
	ReasonCodes      []string `json:"reason_codes,omitempty"`
	RetainedMetadata any      `json:"retained_metadata,omitempty"`
}

type ReplayOptions struct {
	ProjectDir string
	Candidate  CandidateBundle
}

type ReplayResult struct {
	PromotionReady bool           `json:"promotion_ready"`
	Evidence       ReplayEvidence `json:"evidence"`
	FailureReasons []string       `json:"failure_reasons,omitempty"`
	LLMScore       LLMScore       `json:"llm_score"`
}

type ReplayEvidence struct {
	Commands []ReplayCommand       `json:"commands,omitempty"`
	Checks   []ReplayCheckEvidence `json:"checks,omitempty"`
}

type ReplayCheckEvidence struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Deterministic bool   `json:"deterministic"`
	AcceptanceRef string `json:"acceptance_ref,omitempty"`
	Source        string `json:"source,omitempty"`
}

type HumanApproval struct {
	Approved   bool   `json:"approved,omitempty"`
	ApprovedBy string `json:"approved_by,omitempty"`
	ApprovedAt string `json:"approved_at,omitempty"`
}

type PromotionOptions struct {
	ProjectDir string
	Candidate  CandidateBundle
	Approval   HumanApproval
	Apply      bool
}

type PromotionResult struct {
	Applied        bool     `json:"applied"`
	AppliedPaths   []string `json:"applied_paths,omitempty"`
	RequiredChecks []string `json:"required_checks"`
}

type ArchiveOptions struct {
	QuarantineDir string
	Candidate     CandidateBundle
	Reason        string
}

type ArchiveResult struct {
	Status             string              `json:"status"`
	ReasonCode         string              `json:"reason_code"`
	Provenance         CandidateProvenance `json:"provenance"`
	ReplayEvidenceRefs []string            `json:"replay_evidence_refs,omitempty"`
	ArchivePath        string              `json:"archive_path"`
}
