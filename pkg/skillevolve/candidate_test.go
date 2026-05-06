package skillevolve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCandidates_FromRepeatedStructuralOnlyFailures_CreatesQuarantinedBundle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	indexPath := writeRepeatedFailureIndex(t, dir)
	quarantineDir := filepath.Join(dir, ".autopus", "skill-evolve", "quarantine")

	result, err := GenerateCandidates(context.Background(), CandidateGenerationOptions{
		QualityIndexPath: indexPath,
		QuarantineDir:    quarantineDir,
		MinCount:         2,
		Creator:          "tester-agent",
	})

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)

	candidate := result.Candidates[0]
	assert.Equal(t, "oracle.structural_only.missing_semantic_output", candidate.Fingerprint)
	assert.Equal(t, "quarantined", candidate.Status)
	assert.False(t, candidate.Active)
	assert.Equal(t, "tester-agent", candidate.Creator)
	assert.Equal(t, "passed", candidate.RedactionStatus)

	require.Len(t, candidate.SourceFailures, 3)
	sourceRefs := make([]string, 0, len(candidate.SourceFailures))
	for _, failure := range candidate.SourceFailures {
		sourceRefs = append(sourceRefs, failure.Ref)
		assert.Regexp(t, `^sha256:[a-f0-9]{64}$`, failure.Hash)
		assert.NotEmpty(t, failure.EvidenceRef)
	}
	assert.ElementsMatch(t, []string{
		"qamesh-run-1/manifest.json#AC-QAMESH2-006",
		"qamesh-run-2/manifest.json#AC-QAMESH2-006",
		"learn/2026-05-06T02:00:00Z.jsonl#L17",
	}, sourceRefs)
	assert.ElementsMatch(t, []string{fakeSHA("a"), fakeSHA("b"), fakeSHA("c")}, candidate.SourceHashes)
	assert.ElementsMatch(t, []string{
		"autopus-adk/content/skills/testing-strategy.md",
		"autopus-adk/templates/shared/branding-formats.md.tmpl",
	}, candidate.AffectedRefs)
	assert.ElementsMatch(t, []string{"AC-QAMESH2-006", "AC-SEVOLVE-001"}, candidate.AffectedAcceptanceIDs)
	assert.Regexp(t, `^sha256:[a-f0-9]{64}$`, candidate.ProposedDigest)
	assert.Regexp(t, `^sha256:[a-f0-9]{64}$`, candidate.GenerationPromptDigest)

	require.NotEmpty(t, candidate.ReplayPlan.Commands)
	assert.Equal(t, "go test ./pkg/skillevolve -run Replay -count=1", candidate.ReplayPlan.Commands[0].Command)
	assert.ElementsMatch(t, []string{"AC-SEVOLVE-001", "AC-SEVOLVE-003"}, candidate.ReplayPlan.AcceptanceRefs)

	require.NotEmpty(t, candidate.BundlePath)
	assertPathWithin(t, quarantineDir, candidate.BundlePath)
	assert.FileExists(t, candidate.BundlePath)
	body, err := os.ReadFile(candidate.BundlePath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "oracle.structural_only.missing_semantic_output")
	assert.Contains(t, string(body), "AC-SEVOLVE-001")
	assert.Contains(t, string(body), candidate.ProposedDigest)
}

func TestGenerateCandidates_QuarantinesWithoutMutatingCanonicalOrGeneratedSurfaces(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	canonical := "autopus-adk/content/skills/testing-strategy.md"
	generated := []string{
		".agents/skills/testing-strategy.md",
		".codex/skills/testing-strategy.md",
		".opencode/commands/auto-test.md",
		".autopus/plugins/index.json",
		".codex/plugins/cache/autopus-local/auto/1.0.0/skills/auto-go/SKILL.md",
	}
	writeWorkspaceFile(t, projectDir, canonical, "canonical skill before\n")
	for _, rel := range generated {
		writeWorkspaceFile(t, projectDir, rel, "generated surface before: "+rel+"\n")
	}
	before := readWorkspaceFiles(t, projectDir, append([]string{canonical}, generated...))

	result, err := GenerateCandidates(context.Background(), CandidateGenerationOptions{
		ProjectDir:       projectDir,
		QualityIndexPath: writeRepeatedFailureIndex(t, projectDir),
		QuarantineDir:    filepath.Join(projectDir, ".autopus", "skill-evolve", "quarantine"),
		MinCount:         2,
		Creator:          "tester-agent",
	})

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	assert.Equal(t, "quarantined", result.Candidates[0].Status)
	assert.False(t, result.Candidates[0].Active)
	assertPathWithin(t, filepath.Join(projectDir, ".autopus", "skill-evolve", "quarantine"), result.Candidates[0].BundlePath)

	after := readWorkspaceFiles(t, projectDir, append([]string{canonical}, generated...))
	assert.Equal(t, before, after)
}

func TestGenerateCandidates_MarksUnsafeCandidateRejectedBeforePersisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	quarantineDir := filepath.Join(dir, ".autopus", "skill-evolve", "quarantine")

	result, err := GenerateCandidates(context.Background(), CandidateGenerationOptions{
		ProjectDir:       dir,
		QualityIndexPath: writeUnsafeFailureIndex(t, dir),
		QuarantineDir:    quarantineDir,
		MinCount:         2,
		Creator:          "tester-agent",
	})

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	candidate := result.Candidates[0]
	assert.Equal(t, "rejected", candidate.Status)
	assert.False(t, candidate.Active)
	assert.Equal(t, "failed", candidate.RedactionStatus)
	assert.Contains(t, candidate.SafetyReasonCodes, "affected_file_outside_owned_paths")

	body, err := os.ReadFile(candidate.BundlePath)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status": "rejected"`)
	assert.Contains(t, string(body), `"redaction_status": "failed"`)
	assert.Contains(t, string(body), "affected_file_outside_owned_paths")
}

type qualityFailure struct {
	Ref             string   `json:"ref"`
	Fingerprint     string   `json:"fingerprint"`
	SourceHash      string   `json:"source_hash"`
	EvidenceRef     string   `json:"evidence_ref"`
	AffectedRefs    []string `json:"affected_refs"`
	AcceptanceRefs  []string `json:"acceptance_refs"`
	Expected        string   `json:"expected"`
	Actual          string   `json:"actual"`
	FailureSeverity string   `json:"failure_severity"`
}

func writeRepeatedFailureIndex(t *testing.T, dir string) string {
	t.Helper()

	payload := map[string]any{
		"schema_version": "autopus.quality_index.v1",
		"failures": []qualityFailure{
			structuralOnlyFailure("qamesh-run-1/manifest.json#AC-QAMESH2-006", fakeSHA("a")),
			structuralOnlyFailure("qamesh-run-2/manifest.json#AC-QAMESH2-006", fakeSHA("b")),
			structuralOnlyFailure("learn/2026-05-06T02:00:00Z.jsonl#L17", fakeSHA("c")),
			{
				Ref:             "qamesh-run-3/manifest.json#AC-OTHER",
				Fingerprint:     "browser.timeout",
				SourceHash:      fakeSHA("d"),
				EvidenceRef:     "qamesh-run-3/run-index.json",
				AffectedRefs:    []string{"autopus-adk/content/skills/frontend-verify.md"},
				AcceptanceRefs:  []string{"AC-OTHER"},
				Expected:        "stable browser target",
				Actual:          "timeout",
				FailureSeverity: "warn",
			},
		},
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(dir, "quality-index.json")
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
	return path
}

func writeUnsafeFailureIndex(t *testing.T, dir string) string {
	t.Helper()

	payload := map[string]any{
		"schema_version": "autopus.quality_index.v1",
		"failures": []qualityFailure{
			unsafeGeneratedSurfaceFailure("qamesh-run-1/manifest.json#AC-QAMESH2-006", fakeSHA("a")),
			unsafeGeneratedSurfaceFailure("qamesh-run-2/manifest.json#AC-QAMESH2-006", fakeSHA("b")),
		},
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(dir, "unsafe-quality-index.json")
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
	return path
}

func structuralOnlyFailure(ref, hash string) qualityFailure {
	return qualityFailure{
		Ref:         ref,
		Fingerprint: "oracle.structural_only.missing_semantic_output",
		SourceHash:  hash,
		EvidenceRef: "autopus-adk/pkg/skillevolve/testdata/qamesh-run-1/run-index.json",
		AffectedRefs: []string{
			"autopus-adk/content/skills/testing-strategy.md",
			"autopus-adk/templates/shared/branding-formats.md.tmpl",
		},
		AcceptanceRefs:  []string{"AC-QAMESH2-006", "AC-SEVOLVE-001"},
		Expected:        "oracle report includes concrete semantic output rows",
		Actual:          "oracle report only includes headings and section labels",
		FailureSeverity: "must",
	}
}

func unsafeGeneratedSurfaceFailure(ref, hash string) qualityFailure {
	return qualityFailure{
		Ref:             ref,
		Fingerprint:     "generated.surface.direct_edit",
		SourceHash:      hash,
		EvidenceRef:     "autopus-adk/pkg/skillevolve/testdata/qamesh-run-1/run-index.json",
		AffectedRefs:    []string{".codex/skills/testing-strategy.md"},
		AcceptanceRefs:  []string{"AC-SEVOLVE-004"},
		Expected:        "candidate remains outside generated surfaces",
		Actual:          "failure points at generated skill surface",
		FailureSeverity: "must",
	}
}

func fakeSHA(ch string) string {
	return "sha256:" + strings.Repeat(ch, 64)
}

func writeWorkspaceFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func readWorkspaceFiles(t *testing.T, root string, rels []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(rels))
	for _, rel := range rels {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoError(t, err)
		out[rel] = string(body)
	}
	return out
}

func assertPathWithin(t *testing.T, root, path string) {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	require.NoError(t, err)
	assert.False(t, rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)), "%s escapes %s", path, root)
}
