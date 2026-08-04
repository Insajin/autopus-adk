package promptlayer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestEvaluateOMPContextMemoryShadowV1_AdmitsOnlyFreshCurrentProvenance(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	currentHash := ompContextMemoryTestHash("current")
	authority := OMPContextMemoryAuthorityV1{
		Workspace:    "autopus-adk",
		Spec:         "SPEC-OMP-004",
		Role:         "executor",
		Namespace:    "context-v1",
		Now:          now,
		RequiredRefs: []string{"AGENTS.md", ".autopus/specs/SPEC-OMP-004/acceptance.md"},
		CurrentSources: map[string]OMPContextMemoryCurrentSourceV1{
			"docs/current.md": {Hash: currentHash, ChangedAt: now.Add(-time.Minute), Verified: true},
		},
	}
	base := OMPContextMemoryCandidateV1{
		Workspace: "autopus-adk", Spec: "SPEC-OMP-004", Role: "executor",
		Ref: "docs/current.md", SourceHash: currentHash, CheckedAt: now.Add(-30 * time.Second),
		TTL: 10 * time.Minute, Namespace: "context-v1", Body: "safe optional evidence",
	}
	candidates := []OMPContextMemoryCandidateV1{
		withOMPContextMemoryID(base, "fresh"),
		withOMPContextMemoryTime(base, "expired", now.Add(-20*time.Minute)),
		withOMPContextMemoryNamespace(base, "wrong-namespace", "other"),
		withOMPContextMemoryTime(base, "stale-source", now.Add(-2*time.Minute)),
		withOMPContextMemoryHash(base, "tampered", ompContextMemoryTestHash("tampered")),
	}

	result, err := EvaluateOMPContextMemoryShadowV1(authority, candidates)
	if err != nil {
		t.Fatalf("evaluate memory shadow: %v", err)
	}
	if len(result.ShadowAcceptedIDs) != 1 || result.ShadowAcceptedIDs[0] != "fresh" {
		t.Fatalf("shadow accepted IDs = %v", result.ShadowAcceptedIDs)
	}
	if len(result.ActiveInjectedIDs) != 0 {
		t.Fatalf("active memory injection is forbidden: %v", result.ActiveInjectedIDs)
	}
	wantOmissions := []OMPContextMemoryOmissionV1{
		{ID: "expired", Reason: OMPContextMemoryReasonExpiredV1},
		{ID: "stale-source", Reason: OMPContextMemoryReasonSourceStaleV1},
		{ID: "tampered", Reason: OMPContextMemoryReasonHashMismatchV1},
		{ID: "wrong-namespace", Reason: OMPContextMemoryReasonNamespaceMismatchV1},
	}
	if !equalOMPContextMemoryOmissions(result.Omissions, wantOmissions) {
		t.Fatalf("omissions\nwant: %+v\n got: %+v", wantOmissions, result.Omissions)
	}
	if result.CanonicalMutationCount != 0 || result.DeleteActionCount != 0 {
		t.Fatalf("memory evaluation exposed mutations: %+v", result)
	}
	if len(result.AcceptedProvenance) != 1 || result.AcceptedProvenance[0].Ref != "docs/current.md" {
		t.Fatalf("accepted provenance mismatch: %+v", result.AcceptedProvenance)
	}
	if len(result.RequiredRefs) != len(authority.RequiredRefs) {
		t.Fatalf("required refs changed: %v", result.RequiredRefs)
	}
}

func TestEvaluateOMPContextMemoryShadowV1_ResultIsBodyFreeAndDeterministic(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	hash := ompContextMemoryTestHash("current")
	authority := OMPContextMemoryAuthorityV1{
		Workspace: "workspace", Spec: "SPEC-OMP-004", Role: "reviewer", Namespace: "ns", Now: now,
		CurrentSources: map[string]OMPContextMemoryCurrentSourceV1{"docs/a.md": {Hash: hash, ChangedAt: now.Add(-time.Hour), Verified: true}},
	}
	candidate := OMPContextMemoryCandidateV1{
		ID: "a", Workspace: "workspace", Spec: "SPEC-OMP-004", Role: "reviewer", Ref: "docs/a.md",
		SourceHash: hash, CheckedAt: now.Add(-time.Minute), TTL: time.Hour, Namespace: "ns",
		Body: "RAW-MEMORY-BODY",
	}
	first, err := EvaluateOMPContextMemoryShadowV1(authority, []OMPContextMemoryCandidateV1{candidate})
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluateOMPContextMemoryShadowV1(authority, []OMPContextMemoryCandidateV1{candidate})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("memory receipt is nondeterministic\n%s\n%s", firstJSON, secondJSON)
	}
	if containsOMPContextMemoryText(string(firstJSON), "RAW-MEMORY-BODY") {
		t.Fatalf("raw body leaked into memory receipt: %s", firstJSON)
	}
}

func ompContextMemoryTestHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func withOMPContextMemoryID(candidate OMPContextMemoryCandidateV1, id string) OMPContextMemoryCandidateV1 {
	candidate.ID = id
	return candidate
}

func withOMPContextMemoryTime(candidate OMPContextMemoryCandidateV1, id string, checkedAt time.Time) OMPContextMemoryCandidateV1 {
	candidate.ID, candidate.CheckedAt = id, checkedAt
	return candidate
}

func withOMPContextMemoryNamespace(candidate OMPContextMemoryCandidateV1, id, namespace string) OMPContextMemoryCandidateV1 {
	candidate.ID, candidate.Namespace = id, namespace
	return candidate
}

func withOMPContextMemoryHash(candidate OMPContextMemoryCandidateV1, id, hash string) OMPContextMemoryCandidateV1 {
	candidate.ID, candidate.SourceHash = id, hash
	return candidate
}

func equalOMPContextMemoryOmissions(a, b []OMPContextMemoryOmissionV1) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func containsOMPContextMemoryText(value, target string) bool {
	for index := 0; index+len(target) <= len(value); index++ {
		if value[index:index+len(target)] == target {
			return true
		}
	}
	return false
}
