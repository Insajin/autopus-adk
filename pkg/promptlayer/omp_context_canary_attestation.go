package promptlayer

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"
)

const OMPContextPromotionAttestationSchemaV1 = "autopus.omp_context_promotion_attestation.v1"

var (
	ErrOMPContextPromotionAttestationUnavailable = errors.New("OMP context promotion attestation is unavailable")
	ErrOMPContextPromotionAttestationStale       = errors.New("OMP context promotion attestation is stale")
	ErrOMPContextPromotionAttestationMismatch    = errors.New("OMP context promotion attestation mismatches active admission")
	ErrOMPContextPromotionEvidenceRejected       = errors.New("OMP context promotion evidence is rejected")
)

const maxOMPContextPromotionAttestationTTL = 24 * time.Hour

type OMPContextPromotionSubjectV1 struct {
	WorkspaceID string `json:"workspace_id"`
	SpecID      string `json:"spec_id"`
	TaskID      string `json:"task_id"`
	Phase       string `json:"phase"`
	SessionID   string `json:"session_id"`
	BindingHash string `json:"binding_hash"`
}

type OMPContextPromotionPolicyV1 struct {
	Profile             string `json:"profile"`
	HistoryMode         string `json:"history_mode"`
	MemoryMode          string `json:"memory_mode"`
	HistoryTargetTokens int    `json:"history_target_tokens"`
	Fallback            string `json:"fallback"`
	CapabilityPolicy    string `json:"capability_policy"`
	RuntimeRootPolicy   string `json:"runtime_root_policy"`
	MutationScope       string `json:"mutation_scope"`
}

func OMPContextPromotionPolicyDigestV1(policy OMPContextPromotionPolicyV1) (string, error) {
	if err := validateOMPContextPromotionPolicyV1(policy); err != nil {
		return "", err
	}
	return hashOMPContextPromotionValueV1(policy), nil
}

type OMPContextPromotionAttestationInputV1 struct {
	Subject   OMPContextPromotionSubjectV1
	Policy    OMPContextPromotionPolicyV1
	Rows      []OMPContextCanaryRowV1
	CheckedAt time.Time
	ValidFor  time.Duration
}

type OMPContextPromotionAttestationV1 struct {
	schemaVersion     string
	subjectDigest     string
	policyDigest      string
	canaryDigest      string
	attestationDigest string
	checkedAt         time.Time
	expiresAt         time.Time
}

type OMPContextPromotionEvidenceV1 struct {
	Rows        []OMPContextCanaryRowV1
	Attestation OMPContextPromotionAttestationV1
}

func BuildOMPContextPromotionAttestationV1(input OMPContextPromotionAttestationInputV1) (OMPContextPromotionAttestationV1, error) {
	if err := validateOMPContextPromotionSubjectV1(input.Subject); err != nil {
		return OMPContextPromotionAttestationV1{}, err
	}
	if err := validateOMPContextPromotionPolicyV1(input.Policy); err != nil {
		return OMPContextPromotionAttestationV1{}, err
	}
	if input.CheckedAt.IsZero() || input.ValidFor <= 0 || input.ValidFor > maxOMPContextPromotionAttestationTTL {
		return OMPContextPromotionAttestationV1{}, fmt.Errorf("%w: invalid validity window", ErrOMPContextPromotionEvidenceRejected)
	}
	aggregate, err := ReduceOMPContextCanaryPairsV1(input.Rows)
	if err != nil {
		return OMPContextPromotionAttestationV1{}, fmt.Errorf("%w: %v", ErrOMPContextPromotionEvidenceRejected, err)
	}
	decision := evaluateOMPContextHistoryPromotionAggregateV1(
		OMPContextHistoryModeActiveV1, OMPContextHistoryModeShadowV1, OMPContextMemoryModeV1(input.Policy.MemoryMode), aggregate,
	)
	if !decision.Admitted || decision.EffectiveHistoryMode != OMPContextHistoryModeActiveV1 {
		return OMPContextPromotionAttestationV1{}, fmt.Errorf("%w: %s", ErrOMPContextPromotionEvidenceRejected, decision.Reason)
	}
	checkedAt := input.CheckedAt.UTC()
	attestation := OMPContextPromotionAttestationV1{
		schemaVersion: OMPContextPromotionAttestationSchemaV1,
		subjectDigest: hashOMPContextPromotionValueV1(input.Subject),
		policyDigest:  hashOMPContextPromotionValueV1(input.Policy),
		canaryDigest:  hashOMPContextPromotionValueV1(canonicalOMPContextCanaryRowsV1(input.Rows)),
		checkedAt:     checkedAt, expiresAt: checkedAt.Add(input.ValidFor),
	}
	attestation.attestationDigest = hashOMPContextPromotionValueV1(struct {
		SchemaVersion, SubjectDigest, PolicyDigest, CanaryDigest string
		CheckedAt, ExpiresAt                                     string
	}{
		attestation.schemaVersion, attestation.subjectDigest, attestation.policyDigest, attestation.canaryDigest,
		attestation.checkedAt.Format(time.RFC3339Nano), attestation.expiresAt.Format(time.RFC3339Nano),
	})
	return attestation, nil
}

func VerifyOMPContextPromotionAttestationV1(
	attestation OMPContextPromotionAttestationV1,
	subject OMPContextPromotionSubjectV1,
	policy OMPContextPromotionPolicyV1,
	rows []OMPContextCanaryRowV1,
	now time.Time,
) error {
	if attestation.IsZero() {
		return ErrOMPContextPromotionAttestationUnavailable
	}
	if now.IsZero() || now.UTC().Before(attestation.checkedAt.Add(-5*time.Minute)) {
		return ErrOMPContextPromotionAttestationMismatch
	}
	if !now.UTC().Before(attestation.expiresAt) {
		return ErrOMPContextPromotionAttestationStale
	}
	expected, err := BuildOMPContextPromotionAttestationV1(OMPContextPromotionAttestationInputV1{
		Subject: subject, Policy: policy, Rows: rows, CheckedAt: attestation.checkedAt,
		ValidFor: attestation.expiresAt.Sub(attestation.checkedAt),
	})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(attestation, expected) {
		return ErrOMPContextPromotionAttestationMismatch
	}
	return nil
}

func (attestation OMPContextPromotionAttestationV1) IsZero() bool {
	return attestation.attestationDigest == ""
}

func (attestation OMPContextPromotionAttestationV1) Digest() string {
	return attestation.attestationDigest
}

func (attestation OMPContextPromotionAttestationV1) CanaryDigest() string {
	return attestation.canaryDigest
}

func (attestation OMPContextPromotionAttestationV1) PolicyDigest() string {
	return attestation.policyDigest
}

func (attestation OMPContextPromotionAttestationV1) CheckedAt() time.Time {
	return attestation.checkedAt
}

func (attestation OMPContextPromotionAttestationV1) ExpiresAt() time.Time {
	return attestation.expiresAt
}

func validateOMPContextPromotionSubjectV1(subject OMPContextPromotionSubjectV1) error {
	values := []struct{ name, value string }{
		{"workspace_id", subject.WorkspaceID}, {"spec_id", subject.SpecID}, {"task_id", subject.TaskID},
		{"phase", subject.Phase}, {"session_id", subject.SessionID},
	}
	for _, value := range values {
		if _, err := validateOMPContextMetadata(value.name, value.value); err != nil {
			return fmt.Errorf("%w: %v", ErrOMPContextPromotionEvidenceRejected, err)
		}
	}
	if !validOMPContextCanaryHashV1(subject.BindingHash) {
		return fmt.Errorf("%w: invalid binding hash", ErrOMPContextPromotionEvidenceRejected)
	}
	return nil
}

func validateOMPContextPromotionPolicyV1(policy OMPContextPromotionPolicyV1) error {
	if _, err := validateOMPContextMetadata("profile", policy.Profile); err != nil {
		return fmt.Errorf("%w: %v", ErrOMPContextPromotionEvidenceRejected, err)
	}
	if policy.HistoryMode != "active" || !validOMPContextMemoryModeV1(OMPContextMemoryModeV1(policy.MemoryMode)) ||
		policy.HistoryTargetTokens <= 0 || policy.Fallback == "" || policy.CapabilityPolicy == "" ||
		policy.RuntimeRootPolicy == "" || policy.MutationScope == "" {
		return fmt.Errorf("%w: invalid active policy", ErrOMPContextPromotionEvidenceRejected)
	}
	for _, value := range []string{policy.Fallback, policy.CapabilityPolicy, policy.RuntimeRootPolicy, policy.MutationScope} {
		if _, err := validateOMPContextMetadata("policy", value); err != nil {
			return fmt.Errorf("%w: %v", ErrOMPContextPromotionEvidenceRejected, err)
		}
	}
	return nil
}

func canonicalOMPContextCanaryRowsV1(rows []OMPContextCanaryRowV1) []OMPContextCanaryRowV1 {
	ordered := append([]OMPContextCanaryRowV1(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].TaskID != ordered[j].TaskID {
			return ordered[i].TaskID < ordered[j].TaskID
		}
		if ordered[i].Variant != ordered[j].Variant {
			return ordered[i].Variant < ordered[j].Variant
		}
		return ordered[i].Order < ordered[j].Order
	})
	return ordered
}

func hashOMPContextPromotionValueV1(value any) string {
	data, _ := json.Marshal(value)
	return canonicalHash(data)
}
