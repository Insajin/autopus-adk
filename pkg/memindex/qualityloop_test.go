package memindex

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qualityloop"
)

func TestImprovementCandidateProjectionIndexesSafeRowsOnly(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	safe := qualityloop.ImprovementCandidate{
		SchemaVersion:            qualityloop.SchemaVersion,
		CandidateID:              "ic-safe-001",
		CandidateKind:            qualityloop.KindQAMESHRepairHandoff,
		Status:                   qualityloop.StatusAwaitingReplay,
		Active:                   false,
		WorkspaceID:              "ws-alpha",
		OwningRepo:               "Autopus",
		Owner:                    "frontend",
		FailureFingerprint:       "qamesh.checkout.submit.missing",
		FailureTaxonomy:          qualityloop.TaxonomyProductBug,
		ReasonCodes:              []string{"qamesh_failed_check"},
		ClassificationConfidence: 0.93,
		ConfidenceBand:           qualityloop.BandHigh,
		EvidenceStrength:         qualityloop.EvidenceDeterministic,
		EvidenceRefs:             []string{".autopus/qa/runs/qa-safe/manifest.json"},
		SourceRefs:               []string{"qamesh.evidence.v2:qa-safe"},
		SourceHashes:             []string{"sha256:safe001"},
		AffectedAcceptanceIDs:    []string{"AC-QIL-003"},
		AffectedRefs:             []string{"Autopus/frontend/src/components/Checkout.tsx"},
		RecommendedRoute:         qualityloop.KindQAMESHRepairHandoff,
		TargetArtifact:           "Autopus/frontend/src/components/Checkout.tsx",
		ExpectedValidation:       "auto qa feedback and deterministic replay",
		RollbackPath:             "revert target artifact after approval audit",
		RepairActionPolicy:       qualityloop.PolicyReplayRequired,
		ProviderWriteCallCount:   0,
		RedactionStatus:          qualityloop.RedactionMetadataOnly,
		RetentionClass:           qualityloop.RetentionAudit,
		RawPayloadPresent:        false,
		AuditRefs:                []string{"audit:ic-safe-001"},
	}
	writeCandidate(t, projectDir, "safe.json", safe)

	unsafe := safe
	unsafe.CandidateID = "ic-unsafe-001"
	unsafe.RawPayloadPresent = true
	writeCandidate(t, projectDir, "unsafe.json", unsafe)

	unredacted := safe
	unredacted.CandidateID = "ic-unredacted-001"
	unredacted.RedactionStatus = "failed"
	writeCandidate(t, projectDir, "unredacted.json", unredacted)

	unsafeMetadata := safe
	unsafeMetadata.CandidateID = "ic-unsafe-meta-001"
	unsafeMetadata.ReplayEvidenceRefs = []string{"/Users/alice/private/run-index.json"}
	writeCandidate(t, projectDir, "unsafe-meta.json", unsafeMetadata)

	records, skips, err := Scan(projectDir)
	require.NoError(t, err)

	var candidateRecords []Record
	for _, record := range records {
		if record.SourceType == "improvement_candidate" {
			candidateRecords = append(candidateRecords, record)
		}
	}
	require.Len(t, candidateRecords, 1)
	got := candidateRecords[0]
	assert.Equal(t, ".autopus/quality-loop/candidates/safe.json", got.SourceRef)
	assert.Equal(t, "Improvement candidate ic-safe-001", got.Title)
	assert.Contains(t, got.Summary, "qamesh_failed_check")
	assert.NotContains(t, got.Summary, "raw_payload")
	assert.Equal(t, qualityloop.RedactionMetadataOnly, got.RedactionStatus)
	assert.Equal(t, []string{"AC-QIL-003"}, got.AcceptanceIDs)
	assert.Equal(t, []string{"Autopus/frontend/src/components/Checkout.tsx"}, got.FileRefs)
	assert.Equal(t, "high", got.Severity)
	assert.Equal(t, "ic-safe-001", got.SourceMetadata["candidate_id"])
	assert.Equal(t, qualityloop.KindQAMESHRepairHandoff, got.SourceMetadata["candidate_kind"])
	assert.Equal(t, false, got.SourceMetadata["active"])
	assert.Equal(t, []string{"sha256:safe001"}, got.SourceMetadata["source_hashes"])
	assert.NotContains(t, got.SourceMetadata, "evidence_refs")

	assert.Equal(t, 1, countSkip(skips, "unsafe_improvement_candidate"))
	assert.Equal(t, 1, countSkip(skips, "unredacted_improvement_candidate"))
	assert.Equal(t, 1, countSkip(skips, "unsafe_source_text"))
}

func writeCandidate(t *testing.T, projectDir, name string, candidate qualityloop.ImprovementCandidate) {
	t.Helper()
	body, err := json.MarshalIndent(candidate, "", "  ")
	require.NoError(t, err)
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "quality-loop", "candidates", name), string(body)+"\n")
}

func countSkip(skips []Skip, reason string) int {
	count := 0
	for _, skip := range skips {
		if skip.Reason == reason {
			count++
		}
	}
	return count
}
