package promptlayer

import (
	"crypto/subtle"
	"errors"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

const OMPContextPromotionRuntimeSchemaV3 = "autopus.omp_context_promotion_runtime.v3"

// OMPContextPromotionStaticPolicyV3 is release-compiled policy. It excludes
// report, upstream artifact (U), and executable (D) digests to avoid self-hash
// cycles; those values enter only through two independently signed artifacts.
type OMPContextPromotionStaticPolicyV3 struct {
	SchemaVersion                string `json:"schema_version"`
	ProducerRepository           string `json:"producer_repository"`
	ProducerWorkflowRef          string `json:"producer_workflow_ref"`
	CandidateRepository          string `json:"candidate_repository"`
	SourceCommit                 string `json:"source_commit"`
	SourceTree                   string `json:"source_tree"`
	Target                       string `json:"target"`
	AutoVersion                  string `json:"auto_version"`
	PolicyID                     string `json:"policy_id"`
	PolicyDigest                 string `json:"policy_digest"`
	OMPVersion                   string `json:"omp_version"`
	OMPExecutableSHA256          string `json:"omp_executable_sha256"`
	PipelineImplementationDigest string `json:"pipeline_implementation_digest"`
	Provider                     string `json:"provider"`
	ModelScopeDigest             string `json:"model_scope_digest"`
	CohortManifestDigest         string `json:"cohort_manifest_digest"`
	OrderSeed                    string `json:"order_seed"`
	OraclePolicyDigest           string `json:"oracle_policy_digest"`
	ReleaseLineageKeyID          string `json:"release_lineage_key_id"`
	ReleaseLineageHandoff        string `json:"release_lineage_handoff"`
	MinimumRollbackFloor         uint64 `json:"minimum_rollback_floor"`
}

// OMPContextPromotionCurrentRuntimeV3 contains coordinates measured from the
// running binary and its explicitly selected OMP executable.
type OMPContextPromotionCurrentRuntimeV3 struct {
	ExecutableSHA256             string
	SourceCommit                 string
	SourceTree                   string
	Target                       string
	AutoVersion                  string
	OMPVersion                   string
	OMPExecutableSHA256          string
	PipelineImplementationDigest string
	ProviderAuthorityDigest      string
}

// OMPContextPromotionRuntimeBundleV3 is a local, signed promotion authority.
type OMPContextPromotionRuntimeBundleV3 struct {
	ReportBytes             []byte
	AttestationBytes        []byte
	ReleaseLineageBytes     []byte
	ReleaseLineageSignature []byte
	ReleaseKey              companionmanifest.TrustedPublicKeyReceipt
}

type ompContextPromotionReleaseLineageVerifierV3 func(
	[]byte,
	[]byte,
	companionmanifest.OMPContextReleaseLineagePolicy,
) (time.Time, error)

// VerifyOMPContextPromotionRuntimeV3 verifies current active authority without
// deriving any caller-owned expectation from the report.
func VerifyOMPContextPromotionRuntimeV3(bundle OMPContextPromotionRuntimeBundleV3,
	expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
) (VerifiedOMPContextPromotion, error) {
	bound, err := bindOMPContextPromotionCurrentExecutableV3(current)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	current = bound
	return verifyOMPContextPromotionRuntimeV3At(bundle, expected, current, time.Now().UTC())
}

func bindOMPContextPromotionCurrentExecutableV3(
	current OMPContextPromotionCurrentRuntimeV3,
) (OMPContextPromotionCurrentRuntimeV3, error) {
	measured, err := currentOMPContextPromotionExecutableSHA256V3()
	if err != nil {
		return OMPContextPromotionCurrentRuntimeV3{}, err
	}
	if current.ExecutableSHA256 != "" &&
		!sameOMPContextPromotionHashV3(current.ExecutableSHA256, measured) {
		return OMPContextPromotionCurrentRuntimeV3{}, errors.New("OMP context promotion runtime executable changed")
	}
	current.ExecutableSHA256 = measured
	return current, nil
}

func verifyOMPContextPromotionRuntimeV3At(bundle OMPContextPromotionRuntimeBundleV3,
	expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
	now time.Time,
) (VerifiedOMPContextPromotion, error) {
	return verifyOMPContextPromotionRuntimeV3WithLineageAt(
		bundle, expected, current, now,
		func(lineage, signature []byte, policy companionmanifest.OMPContextReleaseLineagePolicy) (time.Time, error) {
			verified, err := companionmanifest.VerifyOMPContextReleaseLineage(
				lineage, signature, policy, bundle.ReleaseKey,
			)
			if err != nil {
				return time.Time{}, err
			}
			expiresAt, ok := verified.ExpiresAt()
			if !ok {
				return time.Time{}, errors.New("OMP context release lineage expiry is unavailable")
			}
			return expiresAt, nil
		},
	)
}

func verifyOMPContextPromotionRuntimeV3WithLineageAt(bundle OMPContextPromotionRuntimeBundleV3,
	expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
	now time.Time,
	verifyLineage ompContextPromotionReleaseLineageVerifierV3,
) (VerifiedOMPContextPromotion, error) {
	if !validOMPContextPromotionStaticPolicyV3(expected) ||
		!matchesOMPContextPromotionCurrentRuntimeV3(expected, current) || now.IsZero() || verifyLineage == nil {
		return VerifiedOMPContextPromotion{}, errors.New("OMP context promotion runtime v3 policy is invalid")
	}
	attestation, err := decodeOMPContextPromotionAttestationV2(bundle.AttestationBytes)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	issuedAt, expiresAt, err := verifyOMPContextPromotionSignatureV2(
		bundle.ReportBytes, attestation, now.UTC(), committedOMPContextPromotionPublicKeysV2(),
		ompContextPromotionRevokedKeysV2,
	)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	report, err := decodeOMPContextPromotionReportV1(bundle.ReportBytes)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	if report.TrustLane != attestation.TrustLane ||
		!matchesOMPContextPromotionStaticPolicyV3(report, expected, current) {
		return VerifiedOMPContextPromotion{}, ErrOMPContextPromotionMismatch
	}
	if err := validateOMPContextPromotionCohortAttestationTimeV2(report, issuedAt); err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	upstream := report.Candidate.ArtifactSHA256
	if !sameOMPContextPromotionHashV3(upstream, report.Runtime.AutoBinarySHA256) {
		return VerifiedOMPContextPromotion{}, ErrOMPContextPromotionMismatch
	}
	lineageExpiresAt, err := verifyLineage(
		bundle.ReleaseLineageBytes, bundle.ReleaseLineageSignature,
		companionmanifest.OMPContextReleaseLineagePolicy{
			Now: now, ExpectedKeyID: expected.ReleaseLineageKeyID,
			ExpectedHandoff:        expected.ReleaseLineageHandoff,
			MinimumRollbackFloor:   expected.MinimumRollbackFloor,
			ExpectedUpstreamSHA256: upstream, ExpectedExecutableSHA256: current.ExecutableSHA256,
			ExpectedSourceRepository: expected.CandidateRepository,
			ExpectedSourceCommit:     current.SourceCommit, ExpectedSourceTree: current.SourceTree,
			ExpectedTarget: current.Target, ExpectedVersion: current.AutoVersion,
		},
	)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	if !now.Before(lineageExpiresAt) {
		return VerifiedOMPContextPromotion{}, errors.New("OMP context release lineage is expired")
	}
	if lineageExpiresAt.Before(expiresAt) {
		expiresAt = lineageExpiresAt
	}
	return verifiedOMPContextPromotionFromReportV2(report, attestation.ReportSHA256, expiresAt), nil
}

func validOMPContextPromotionStaticPolicyV3(value OMPContextPromotionStaticPolicyV3) bool {
	return value.SchemaVersion == OMPContextPromotionRuntimeSchemaV3 &&
		safeOMPContextMemoryMetadataV1(value.ProducerRepository) &&
		safeOMPContextMemoryMetadataV1(value.ProducerWorkflowRef) &&
		safeOMPContextMemoryMetadataV1(value.CandidateRepository) &&
		validOMPContextEvidenceGitHashV1(value.SourceCommit) && validOMPContextEvidenceGitHashV1(value.SourceTree) &&
		safeOMPContextMemoryMetadataV1(value.Target) && safeOMPContextMemoryMetadataV1(value.AutoVersion) &&
		safeOMPContextMemoryMetadataV1(value.PolicyID) && validOMPContextMemoryHashV1(value.PolicyDigest) &&
		safeOMPContextMemoryMetadataV1(value.OMPVersion) && validOMPContextMemoryHashV1(value.OMPExecutableSHA256) &&
		validOMPContextMemoryHashV1(value.PipelineImplementationDigest) && safeOMPContextMemoryMetadataV1(value.Provider) &&
		validOMPContextMemoryHashV1(value.ModelScopeDigest) && validOMPContextMemoryHashV1(value.CohortManifestDigest) &&
		validOMPContextMemoryHashV1(value.OrderSeed) && validOMPContextMemoryHashV1(value.OraclePolicyDigest) &&
		safeOMPContextMemoryMetadataV1(value.ReleaseLineageKeyID) &&
		safeOMPContextMemoryMetadataV1(value.ReleaseLineageHandoff) && value.MinimumRollbackFloor > 0
}

func matchesOMPContextPromotionCurrentRuntimeV3(expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
) bool {
	return validOMPContextMemoryHashV1(current.ExecutableSHA256) &&
		validOMPContextMemoryHashV1(current.ProviderAuthorityDigest) &&
		current.SourceCommit == expected.SourceCommit && current.SourceTree == expected.SourceTree &&
		current.Target == expected.Target && current.AutoVersion == expected.AutoVersion &&
		current.OMPVersion == expected.OMPVersion && current.OMPExecutableSHA256 == expected.OMPExecutableSHA256 &&
		current.PipelineImplementationDigest == expected.PipelineImplementationDigest
}

func matchesOMPContextPromotionStaticPolicyV3(report OMPContextPromotionReportV1,
	expected OMPContextPromotionStaticPolicyV3,
	current OMPContextPromotionCurrentRuntimeV3,
) bool {
	return report.Producer.Repository == expected.ProducerRepository &&
		report.Producer.WorkflowRef == expected.ProducerWorkflowRef && report.Candidate.Repository == expected.CandidateRepository &&
		report.Candidate.Revision == current.SourceCommit && report.Candidate.TreeSHA == current.SourceTree &&
		report.Policy.PolicyID == expected.PolicyID && report.Policy.PolicyDigest == expected.PolicyDigest &&
		report.Runtime.AutoVersion == current.AutoVersion && report.Runtime.OMPVersion == current.OMPVersion &&
		report.Runtime.OMPExecutableSHA256 == current.OMPExecutableSHA256 &&
		report.Runtime.PipelineImplementationDigest == current.PipelineImplementationDigest &&
		ompContextPromotionProviderAuthorityDigestV1(report) == current.ProviderAuthorityDigest &&
		report.Provider == expected.Provider && report.ModelScopeDigest == expected.ModelScopeDigest &&
		report.CohortManifestDigest == expected.CohortManifestDigest && report.OrderSeed == expected.OrderSeed &&
		report.OraclePolicyDigest == expected.OraclePolicyDigest
}

func sameOMPContextPromotionHashV3(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
