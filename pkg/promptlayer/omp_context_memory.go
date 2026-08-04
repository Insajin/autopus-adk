package promptlayer

import (
	"fmt"
	"sort"
	"time"
)

const OMPContextMemoryShadowSchemaV1 = "autopus.omp_context_memory_shadow.v1"

const (
	OMPContextMemoryReasonExpiredV1           = "expired"
	OMPContextMemoryReasonNamespaceMismatchV1 = "namespace-mismatch"
	OMPContextMemoryReasonSourceStaleV1       = "source-stale"
	OMPContextMemoryReasonHashMismatchV1      = "hash-mismatch"
	OMPContextMemoryReasonMalformedV1         = "malformed"
	OMPContextMemoryReasonSecretV1            = "secret"
	OMPContextMemoryReasonPromptInjectionV1   = "prompt-injection"
)

type OMPContextMemoryCandidateV1 struct {
	ID         string        `json:"id"`
	Workspace  string        `json:"workspace"`
	Spec       string        `json:"spec"`
	Role       string        `json:"role"`
	Ref        string        `json:"ref"`
	SourceHash string        `json:"source_hash"`
	CheckedAt  time.Time     `json:"checked_at"`
	TTL        time.Duration `json:"ttl"`
	Namespace  string        `json:"namespace"`
	Body       string        `json:"-"`
}

type OMPContextMemoryCurrentSourceV1 struct {
	Hash      string    `json:"hash"`
	ChangedAt time.Time `json:"changed_at"`
	Verified  bool      `json:"verified"`
}

type OMPContextMemoryAuthorityV1 struct {
	Workspace      string                                     `json:"workspace"`
	Spec           string                                     `json:"spec"`
	Role           string                                     `json:"role"`
	Namespace      string                                     `json:"namespace"`
	Now            time.Time                                  `json:"now"`
	RequiredRefs   []string                                   `json:"required_refs"`
	CurrentSources map[string]OMPContextMemoryCurrentSourceV1 `json:"current_sources"`
}

type OMPContextMemoryProvenanceV1 struct {
	ID         string    `json:"id"`
	Workspace  string    `json:"workspace"`
	Spec       string    `json:"spec"`
	Role       string    `json:"role"`
	Ref        string    `json:"ref"`
	SourceHash string    `json:"source_hash"`
	CheckedAt  time.Time `json:"checked_at"`
	TTLSeconds int64     `json:"ttl_seconds"`
	Namespace  string    `json:"namespace"`
}

type OMPContextMemoryOmissionV1 struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type OMPContextMemoryShadowResultV1 struct {
	SchemaVersion          string                         `json:"schema_version"`
	ShadowAcceptedIDs      []string                       `json:"shadow_accepted_ids"`
	ActiveInjectedIDs      []string                       `json:"active_injected_ids"`
	AcceptedProvenance     []OMPContextMemoryProvenanceV1 `json:"accepted_provenance"`
	Omissions              []OMPContextMemoryOmissionV1   `json:"omissions"`
	RequiredRefs           []string                       `json:"required_refs"`
	CanonicalMutationCount int                            `json:"canonical_mutation_count"`
	DeleteActionCount      int                            `json:"delete_action_count"`
}

func EvaluateOMPContextMemoryShadowV1(authority OMPContextMemoryAuthorityV1, candidates []OMPContextMemoryCandidateV1) (OMPContextMemoryShadowResultV1, error) {
	result := OMPContextMemoryShadowResultV1{
		SchemaVersion:      OMPContextMemoryShadowSchemaV1,
		ShadowAcceptedIDs:  []string{},
		ActiveInjectedIDs:  []string{},
		AcceptedProvenance: []OMPContextMemoryProvenanceV1{},
		Omissions:          []OMPContextMemoryOmissionV1{},
		RequiredRefs:       []string{},
	}
	if err := validateOMPContextMemoryAuthorityV1(authority); err != nil {
		return result, err
	}
	result.RequiredRefs = append([]string(nil), authority.RequiredRefs...)
	counts := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		counts[candidate.ID]++
	}
	for index, candidate := range candidates {
		outputID := safeOMPContextMemoryOutputIDV1(candidate.ID, index)
		if counts[candidate.ID] > 1 {
			result.Omissions = append(result.Omissions, OMPContextMemoryOmissionV1{ID: outputID, Reason: OMPContextMemoryReasonMalformedV1})
			continue
		}
		reason := omitOMPContextMemoryCandidateV1(authority, candidate)
		if reason != "" {
			result.Omissions = append(result.Omissions, OMPContextMemoryOmissionV1{ID: outputID, Reason: reason})
			continue
		}
		result.ShadowAcceptedIDs = append(result.ShadowAcceptedIDs, candidate.ID)
		result.AcceptedProvenance = append(result.AcceptedProvenance, OMPContextMemoryProvenanceV1{
			ID: candidate.ID, Workspace: candidate.Workspace, Spec: candidate.Spec, Role: candidate.Role,
			Ref: candidate.Ref, SourceHash: candidate.SourceHash, CheckedAt: candidate.CheckedAt.UTC(),
			TTLSeconds: int64(candidate.TTL / time.Second), Namespace: candidate.Namespace,
		})
	}
	sort.Strings(result.ShadowAcceptedIDs)
	sort.Slice(result.AcceptedProvenance, func(i, j int) bool {
		return result.AcceptedProvenance[i].ID < result.AcceptedProvenance[j].ID
	})
	sort.Slice(result.Omissions, func(i, j int) bool {
		if result.Omissions[i].ID == result.Omissions[j].ID {
			return result.Omissions[i].Reason < result.Omissions[j].Reason
		}
		return result.Omissions[i].ID < result.Omissions[j].ID
	})
	return result, nil
}

func validateOMPContextMemoryAuthorityV1(authority OMPContextMemoryAuthorityV1) error {
	for field, value := range map[string]string{
		"workspace": authority.Workspace, "spec": authority.Spec, "role": authority.Role, "namespace": authority.Namespace,
	} {
		if !safeOMPContextMemoryMetadataV1(value) {
			return fmt.Errorf("OMP memory authority %s is malformed", field)
		}
	}
	if authority.Now.IsZero() {
		return fmt.Errorf("OMP memory authority time is missing")
	}
	seen := make(map[string]bool, len(authority.RequiredRefs))
	for _, ref := range authority.RequiredRefs {
		if !safeOMPContextMemoryRefV1(ref) || seen[ref] {
			return fmt.Errorf("OMP memory required refs are malformed")
		}
		seen[ref] = true
	}
	for ref, source := range authority.CurrentSources {
		if !safeOMPContextMemoryRefV1(ref) || !validOMPContextMemoryHashV1(source.Hash) || source.ChangedAt.IsZero() || source.ChangedAt.After(authority.Now) || !source.Verified {
			return fmt.Errorf("OMP memory current source is malformed")
		}
	}
	return nil
}

// @AX:WARN [AUTO]: memory-candidate rejection has cyclomatic complexity 25.
// @AX:REASON [AUTO]: gocyclo reports 25 across authority, provenance, time, size, sensitivity, and scope invariants.
func omitOMPContextMemoryCandidateV1(authority OMPContextMemoryAuthorityV1, candidate OMPContextMemoryCandidateV1) string {
	if !safeOMPContextMemoryMetadataV1(candidate.ID) || !safeOMPContextMemoryMetadataV1(candidate.Workspace) ||
		!safeOMPContextMemoryMetadataV1(candidate.Spec) || !safeOMPContextMemoryMetadataV1(candidate.Role) ||
		!safeOMPContextMemoryMetadataV1(candidate.Namespace) || !safeOMPContextMemoryRefV1(candidate.Ref) ||
		!validOMPContextMemoryHashV1(candidate.SourceHash) || candidate.CheckedAt.IsZero() ||
		candidate.CheckedAt.After(authority.Now) || candidate.TTL <= 0 || candidate.TTL%time.Second != 0 || candidate.Body == "" || len(candidate.Body) > 32*1024 {
		return OMPContextMemoryReasonMalformedV1
	}
	if hasOMPContextMemoryAbsolutePathV1(candidate.Body) {
		return OMPContextMemoryReasonMalformedV1
	}
	if hasOMPContextMemoryPromptInjectionV1(candidate.Body) {
		return OMPContextMemoryReasonPromptInjectionV1
	}
	if hasOMPContextMemorySecretV1(candidate.Body) {
		return OMPContextMemoryReasonSecretV1
	}
	if candidate.Workspace != authority.Workspace || candidate.Spec != authority.Spec || candidate.Role != authority.Role || candidate.Namespace != authority.Namespace {
		return OMPContextMemoryReasonNamespaceMismatchV1
	}
	if authority.Now.After(candidate.CheckedAt.Add(candidate.TTL)) {
		return OMPContextMemoryReasonExpiredV1
	}
	source, exists := authority.CurrentSources[candidate.Ref]
	if !exists || candidate.CheckedAt.Before(source.ChangedAt) {
		return OMPContextMemoryReasonSourceStaleV1
	}
	if candidate.SourceHash != source.Hash {
		return OMPContextMemoryReasonHashMismatchV1
	}
	return ""
}
