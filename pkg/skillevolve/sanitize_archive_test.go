package skillevolve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSanitizeCandidateStripsContentButKeepsPaths asserts sanitization blanks file
// content while preserving the proposed paths and other metadata.
func TestSanitizeCandidateStripsContentButKeepsPaths(t *testing.T) {
	t.Parallel()

	candidate := CandidateBundle{
		ID: "cand-1",
		ProposedFiles: []ProposedFile{
			{Path: "content/skills/a.md", Content: "secret body"},
			{Path: "content/skills/b.md", Content: "another body"},
		},
	}

	sanitized := SanitizeCandidate(candidate)

	require.Len(t, sanitized.ProposedFiles, 2)
	assert.Equal(t, "content/skills/a.md", sanitized.ProposedFiles[0].Path)
	assert.Empty(t, sanitized.ProposedFiles[0].Content)
	assert.Empty(t, sanitized.ProposedFiles[1].Content)
	// Original candidate must be unchanged (defensive copy).
	assert.Equal(t, "secret body", candidate.ProposedFiles[0].Content)
}

// TestSanitizeGenerationResultSanitizesEveryCandidate asserts the batch helper
// strips content across all candidates.
func TestSanitizeGenerationResultSanitizesEveryCandidate(t *testing.T) {
	t.Parallel()

	result := CandidateGenerationResult{
		Candidates: []CandidateBundle{
			{ID: "a", ProposedFiles: []ProposedFile{{Path: "p1", Content: "x"}}},
			{ID: "b", ProposedFiles: []ProposedFile{{Path: "p2", Content: "y"}}},
		},
	}

	sanitized := SanitizeGenerationResult(result)

	require.Len(t, sanitized.Candidates, 2)
	for _, c := range sanitized.Candidates {
		for _, f := range c.ProposedFiles {
			assert.Empty(t, f.Content)
			assert.NotEmpty(t, f.Path)
		}
	}
}

// TestEvaluateSafetyAllowsCleanCandidate asserts a clean, owned-path candidate is
// admitted with replay and promotion allowed and no reason codes.
func TestEvaluateSafetyAllowsCleanCandidate(t *testing.T) {
	t.Parallel()

	candidate := CandidateBundle{
		ID:         "cand-clean",
		OwnedPaths: []string{"autopus-adk/content/skills/**"},
		ProposedFiles: []ProposedFile{{
			Path:    "autopus-adk/content/skills/testing-strategy.md",
			Content: validSkillContent("testing-strategy"),
		}},
	}

	result, err := EvaluateSafety(context.Background(), candidate, SafetyOptions{})
	require.NoError(t, err)

	assert.True(t, result.Allowed)
	assert.True(t, result.ReplayAllowed)
	assert.True(t, result.PromotionAllowed)
	assert.Empty(t, result.ReasonCodes)
}

// TestEvaluateSafetyFlagsOutsideOwnedPaths asserts a file outside the owned paths
// is rejected with the matching reason code.
func TestEvaluateSafetyFlagsOutsideOwnedPaths(t *testing.T) {
	t.Parallel()

	candidate := CandidateBundle{
		ID:         "cand-outside",
		OwnedPaths: []string{"autopus-adk/content/skills/**"},
		ProposedFiles: []ProposedFile{{
			Path:    "autopus-adk/cmd/auto/main.go",
			Content: "package main\n",
		}},
	}

	result, err := EvaluateSafety(context.Background(), candidate, SafetyOptions{})
	require.NoError(t, err)

	assert.False(t, result.Allowed)
	assert.Contains(t, result.ReasonCodes, "affected_file_outside_owned_paths")
}

// TestEvaluateSafetyHonorsContextCancellation asserts a cancelled context errors.
func TestEvaluateSafetyHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := EvaluateSafety(ctx, CandidateBundle{}, SafetyOptions{})
	require.Error(t, err)
}

// TestArchiveCandidateWritesRecordWithProvenance asserts the archive writes a JSON
// record carrying reason code, provenance derived from source failures, and refs.
func TestArchiveCandidateWritesRecordWithProvenance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	candidate := CandidateBundle{
		ID: "cand-archive",
		SourceFailures: []SourceFailure{
			{Ref: "fail-1", Hash: "h1", EvidenceRef: "evid-1"},
		},
		SourceHashes:           []string{"h1"},
		GenerationPromptDigest: "digest-1",
		Creator:                "evolver",
		ReplayEvidenceRefs:     []string{"replay-evid-1"},
	}

	result, err := ArchiveCandidate(context.Background(), ArchiveOptions{
		QuarantineDir: dir,
		Candidate:     candidate,
		Reason:        "stale_source",
	})
	require.NoError(t, err)

	assert.Equal(t, "archived", result.Status)
	assert.Equal(t, "stale_source", result.ReasonCode)
	assert.Equal(t, []string{"replay-evid-1"}, result.ReplayEvidenceRefs)
	assert.Contains(t, result.Provenance.SourceFailureRefs, "fail-1")
	assert.Contains(t, result.Provenance.EvidenceRefs, "evid-1")
	assert.Equal(t, "digest-1", result.Provenance.GenerationPromptDigest)
	assert.Equal(t, "evolver", result.Provenance.Creator)

	body, err := os.ReadFile(result.ArchivePath)
	require.NoError(t, err)
	var record archiveRecord
	require.NoError(t, json.Unmarshal(body, &record))
	assert.Equal(t, "cand-archive", record.CandidateID)
	assert.Equal(t, "stale_source", record.ReasonCode)
}

// TestArchiveCandidateRequiresReason asserts an empty reason errors.
func TestArchiveCandidateRequiresReason(t *testing.T) {
	t.Parallel()

	_, err := ArchiveCandidate(context.Background(), ArchiveOptions{
		QuarantineDir: t.TempDir(),
		Candidate:     CandidateBundle{ID: "x"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")
}

// TestArchiveCandidateDerivesIDWhenMissing asserts a missing ID gets a digest-based
// name and the archive file is written under the quarantine archive dir.
func TestArchiveCandidateDerivesIDWhenMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result, err := ArchiveCandidate(context.Background(), ArchiveOptions{
		QuarantineDir: dir,
		Candidate:     CandidateBundle{ProposedFiles: []ProposedFile{{Path: "p"}}},
		Reason:        "rejected",
	})
	require.NoError(t, err)

	assert.Contains(t, result.ArchivePath, filepath.Join(dir, "archive"))
	assert.FileExists(t, result.ArchivePath)
}

// TestArchiveCandidateHonorsContextCancellation asserts cancellation errors.
func TestArchiveCandidateHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ArchiveCandidate(ctx, ArchiveOptions{Reason: "r", Candidate: CandidateBundle{ID: "x"}})
	require.Error(t, err)
}
