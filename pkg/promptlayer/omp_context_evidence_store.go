package promptlayer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

const (
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: evidence is capped at 1 MiB, 512 canary rows, and 4096 body-free history references.
	OMPContextEvidenceStoreSchemaV1 = "autopus.omp_context_evidence_store.v1"
	ompContextEvidenceStoreRelV1    = ".autopus/runtime/omp-context/evidence-v1.json"
	ompContextEvidenceStoreMaxV1    = 1 << 20
	ompContextEvidenceCanaryMaxV1   = 512
	ompContextEvidenceHistoryMaxV1  = 4096
)

type OMPContextEvidenceStoreBindingV1 struct {
	WorkspaceID    string        `json:"workspace_id"`
	SpecID         string        `json:"spec_id"`
	SnapshotHash   string        `json:"snapshot_hash"`
	GitCommitHash  string        `json:"git_commit_hash"`
	PolicyDigest   string        `json:"policy_digest"`
	RuntimeVersion string        `json:"runtime_version"`
	CheckedAt      time.Time     `json:"checked_at"`
	ValidFor       time.Duration `json:"valid_for"`
}

type OMPContextEvidenceExpectationV1 struct {
	WorkspaceID    string
	SpecID         string
	SnapshotHash   string
	GitCommitHash  string
	RuntimeVersion string
}

type OMPContextEvidenceStoreV1 struct {
	SchemaVersion string                           `json:"schema_version,omitempty"`
	Binding       OMPContextEvidenceStoreBindingV1 `json:"binding"`
	Policy        OMPContextPromotionPolicyV1      `json:"policy"`
	CanaryRows    []OMPContextCanaryRowV1          `json:"canary_rows"`
	HistoryRefs   []OMPContextHistoryReference     `json:"history_refs"`
}

type OMPContextVerifiedEvidenceStoreV1 struct {
	Promotion   OMPContextPromotionEvidenceV1
	HistoryRefs []OMPContextHistoryReference
}

func OMPContextEvidenceStorePath(root string) string {
	return filepath.Join(root, filepath.FromSlash(ompContextEvidenceStoreRelV1))
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: atomic write is the sole persisted promotion-evidence boundary.
// @AX:REASON [AUTO]: Policy digests, canonical evidence, rooted path checks, companion manifests, and mode verification converge here.
func WriteOMPContextEvidenceStoreV1(root string, document OMPContextEvidenceStoreV1) error {
	// PolicyDigest is derived here and is never accepted as caller authority.
	document.Binding.PolicyDigest = hashOMPContextPromotionValueV1(document.Policy)
	canonical, err := canonicalOMPContextEvidenceStoreV1(document)
	if err != nil {
		return err
	}
	body, err := marshalOMPContextEvidenceStoreV1(canonical)
	if err != nil {
		return err
	}
	if len(body) > ompContextEvidenceStoreMaxV1 {
		return fmt.Errorf("OMP context evidence exceeds size limit")
	}
	path, err := prepareOMPContextEvidenceStorePathV1(root, true)
	if err != nil {
		return err
	}
	if err := companionmanifest.WriteAtomic(path, body); err != nil {
		return fmt.Errorf("write OMP context evidence: %w", err)
	}
	return verifyOMPContextEvidenceFileV1(path)
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: only canary evidence that admits active history may enter the canonical store.
func canonicalOMPContextEvidenceStoreV1(document OMPContextEvidenceStoreV1) (OMPContextEvidenceStoreV1, error) {
	if document.SchemaVersion != "" && document.SchemaVersion != OMPContextEvidenceStoreSchemaV1 {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("invalid OMP context evidence schema")
	}
	document.SchemaVersion = OMPContextEvidenceStoreSchemaV1
	document.Binding.CheckedAt = document.Binding.CheckedAt.UTC()
	if err := validateOMPContextEvidenceBindingV1(document.Binding); err != nil {
		return OMPContextEvidenceStoreV1{}, err
	}
	if err := validateOMPContextEvidencePolicyV1(document.Policy); err != nil {
		return OMPContextEvidenceStoreV1{}, err
	}
	rows := canonicalOMPContextCanaryRowsV1(document.CanaryRows)
	if len(rows) == 0 || len(rows) > ompContextEvidenceCanaryMaxV1 {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("invalid OMP context canary evidence count")
	}
	aggregate, err := ReduceOMPContextCanaryPairsV1(rows)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("invalid OMP context canary evidence: %w", err)
	}
	decision := evaluateOMPContextHistoryPromotionAggregateV1(
		OMPContextHistoryModeActiveV1, OMPContextHistoryModeShadowV1,
		OMPContextMemoryModeV1(document.Policy.MemoryMode), aggregate,
	)
	if !decision.Admitted {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("OMP context canary evidence rejected: %s", decision.Reason)
	}
	history, err := canonicalOMPContextEvidenceHistoryV1(document.HistoryRefs)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, err
	}
	document.CanaryRows = rows
	document.HistoryRefs = history
	return document, nil
}

func validateOMPContextEvidenceBindingV1(binding OMPContextEvidenceStoreBindingV1) error {
	values := []struct{ name, value string }{
		{"workspace_id", binding.WorkspaceID},
		{"spec_id", binding.SpecID},
		{"runtime_version", binding.RuntimeVersion},
	}
	for _, item := range values {
		if !safeOMPContextMemoryMetadataV1(item.value) {
			return fmt.Errorf("invalid OMP context evidence %s", item.name)
		}
	}
	if !validOMPContextMemoryHashV1(binding.SnapshotHash) || !validOMPContextMemoryHashV1(binding.PolicyDigest) {
		return fmt.Errorf("invalid OMP context evidence binding hash")
	}
	if !validOMPContextEvidenceGitHashV1(binding.GitCommitHash) {
		return fmt.Errorf("invalid OMP context evidence git commit")
	}
	if binding.CheckedAt.IsZero() || binding.ValidFor <= 0 || binding.ValidFor > maxOMPContextPromotionAttestationTTL {
		return fmt.Errorf("invalid OMP context evidence validity window")
	}
	return nil
}

func validateOMPContextEvidencePolicyV1(policy OMPContextPromotionPolicyV1) error {
	if err := validateOMPContextPromotionPolicyV1(policy); err != nil {
		return err
	}
	values := []string{policy.Profile, policy.HistoryMode, policy.MemoryMode, policy.Fallback,
		policy.CapabilityPolicy, policy.RuntimeRootPolicy, policy.MutationScope}
	for _, value := range values {
		if !safeOMPContextMemoryMetadataV1(value) {
			return fmt.Errorf("invalid OMP context evidence policy metadata")
		}
	}
	if policy.HistoryTargetTokens > 1_000_000_000 {
		return fmt.Errorf("invalid OMP context history target")
	}
	return nil
}

func canonicalOMPContextEvidenceHistoryV1(refs []OMPContextHistoryReference) ([]OMPContextHistoryReference, error) {
	if len(refs) > ompContextEvidenceHistoryMaxV1 {
		return nil, fmt.Errorf("too many OMP context history references")
	}
	result := append([]OMPContextHistoryReference(nil), refs...)
	seen := make(map[string]bool, len(result))
	for _, ref := range result {
		if !safeOMPContextMemoryMetadataV1(ref.ID) || !safeOMPContextMemoryRefV1(ref.SourceRef) ||
			!validOMPContextMemoryHashV1(ref.BodyHash) || ref.TokenEstimate <= 0 ||
			ref.TokenEstimate > 1_000_000_000 || ref.Reason != "completed-superseded" || seen[ref.ID] {
			return nil, fmt.Errorf("invalid or duplicate OMP context history reference")
		}
		seen[ref.ID] = true
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func validOMPContextEvidenceGitHashV1(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func validOMPContextEvidenceExpectationV1(value OMPContextEvidenceExpectationV1) bool {
	return safeOMPContextMemoryMetadataV1(value.WorkspaceID) &&
		safeOMPContextMemoryMetadataV1(value.SpecID) &&
		safeOMPContextMemoryMetadataV1(value.RuntimeVersion) &&
		validOMPContextMemoryHashV1(value.SnapshotHash) &&
		validOMPContextEvidenceGitHashV1(value.GitCommitHash)
}

func matchesOMPContextEvidenceExpectationV1(
	binding OMPContextEvidenceStoreBindingV1,
	expected OMPContextEvidenceExpectationV1,
) bool {
	return validOMPContextEvidenceExpectationV1(expected) &&
		binding.WorkspaceID == expected.WorkspaceID && binding.SpecID == expected.SpecID &&
		binding.SnapshotHash == expected.SnapshotHash && binding.GitCommitHash == expected.GitCommitHash &&
		binding.RuntimeVersion == expected.RuntimeVersion
}
