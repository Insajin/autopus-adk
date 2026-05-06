package skillevolve

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayCandidate_UsesDeterministicQAMESHFixtureAndTreatsLLMAsAdvisory(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("testdata", "qamesh-run-1")
	candidate := CandidateBundle{
		ID:     "cand-structural-only",
		Status: "quarantined",
		SourceHashes: []string{
			fakeSHA("a"),
		},
		AffectedRefs:          []string{"autopus-adk/content/skills/testing-strategy.md"},
		AffectedAcceptanceIDs: []string{"AC-SEVOLVE-003"},
		ReplayPlan: ReplayPlan{
			RunIndexPath: filepath.Join(fixtureDir, "run-index.json"),
			Commands: []ReplayCommand{{
				Command: "go test ./pkg/skillevolve -run Replay -count=1",
			}},
			MustChecks: []ReplayCheckRef{{
				ID:            "must-semantic-output",
				AcceptanceRef: "AC-SEVOLVE-003",
				Source:        "qamesh-run-1/run-index.json",
			}},
		},
		ProposedFiles: []ProposedFile{{
			Path:    "autopus-adk/content/skills/testing-strategy.md",
			Content: validSkillContent("testing-strategy"),
		}},
		LLMScore: LLMScore{
			Score:     0.98,
			Advisory:  true,
			Authority: "advisory",
		},
		Provenance: CandidateProvenance{
			SourceHashes:          []string{fakeSHA("a")},
			EvidenceRefs:          []string{filepath.Join(fixtureDir, "run-index.json")},
			AffectedAcceptanceIDs: []string{"AC-SEVOLVE-003"},
		},
		ReplayEvidenceRefs: []string{filepath.Join(fixtureDir, "run-index.json")},
	}

	result, err := ReplayCandidate(context.Background(), ReplayOptions{Candidate: candidate})

	require.NoError(t, err)
	assert.True(t, result.PromotionReady)
	require.NotEmpty(t, result.Evidence.Commands)
	assert.Equal(t, "go test ./pkg/skillevolve -run Replay -count=1", result.Evidence.Commands[0].Command)
	require.NotEmpty(t, result.Evidence.Checks)
	assert.Equal(t, "must-semantic-output", result.Evidence.Checks[0].ID)
	assert.Equal(t, "passed", result.Evidence.Checks[0].Status)
	assert.True(t, result.Evidence.Checks[0].Deterministic)
	assert.Equal(t, "AC-SEVOLVE-003", result.Evidence.Checks[0].AcceptanceRef)
	assert.True(t, result.LLMScore.Advisory)
	assert.Equal(t, "advisory", result.LLMScore.Authority)
}

func TestReplayCandidate_LLMOnlyScoreCannotMakePromotionReady(t *testing.T) {
	t.Parallel()

	result, err := ReplayCandidate(context.Background(), ReplayOptions{
		Candidate: CandidateBundle{
			ID:     "cand-llm-only",
			Status: "quarantined",
			LLMScore: LLMScore{
				Score:     1.0,
				Advisory:  true,
				Authority: "advisory",
			},
		},
	})

	require.NoError(t, err)
	assert.False(t, result.PromotionReady)
	assert.Contains(t, result.FailureReasons, "deterministic_must_checks_missing")
	assert.True(t, result.LLMScore.Advisory)
	assert.Equal(t, "advisory", result.LLMScore.Authority)
}

func TestReplayCandidate_RejectsRunIndexOutsideProjectDir(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideRunIndex := writeSkillEvolvePackageRunIndex(t, outsideDir, "passed")

	result, err := ReplayCandidate(context.Background(), ReplayOptions{
		ProjectDir: projectDir,
		Candidate: CandidateBundle{
			ID:     "cand-outside-replay",
			Status: "quarantined",
			ReplayPlan: ReplayPlan{
				RunIndexPath: outsideRunIndex,
				MustChecks:   []ReplayCheckRef{{ID: "must-semantic-output"}},
			},
		},
	})

	require.NoError(t, err)
	assert.False(t, result.PromotionReady)
	assert.Contains(t, result.FailureReasons, "replay_run_index_outside_project")
}

func TestPromoteCandidate_ApplyRequiresExplicitApprovalAndOnlyADKSourcePaths(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	sourceRel := "autopus-adk/content/skills/testing-strategy.md"
	generatedRel := ".codex/skills/testing-strategy.md"
	oldSource := validSkillContent("testing-strategy") + "old guidance\n"
	newSource := validSkillContent("testing-strategy") + "Require semantic oracle output rows.\n"
	writeWorkspaceFile(t, projectDir, sourceRel, oldSource)
	writeWorkspaceFile(t, projectDir, generatedRel, "generated surface before\n")

	candidate := promotionReadyCandidate(newSource)

	dryRun, err := PromoteCandidate(context.Background(), PromotionOptions{
		ProjectDir: projectDir,
		Candidate:  candidate,
		Approval:   HumanApproval{ApprovedBy: "human-reviewer"},
		Apply:      false,
	})
	require.NoError(t, err)
	assert.False(t, dryRun.Applied)
	assert.Equal(t, oldSource, readWorkspaceFiles(t, projectDir, []string{sourceRel})[sourceRel])

	applied, err := PromoteCandidate(context.Background(), PromotionOptions{
		ProjectDir: projectDir,
		Candidate:  candidate,
		Approval:   HumanApproval{ApprovedBy: "human-reviewer"},
		Apply:      true,
	})

	require.NoError(t, err)
	assert.True(t, applied.Applied)
	assert.Equal(t, []string{sourceRel}, applied.AppliedPaths)
	assert.Contains(t, applied.RequiredChecks, "template_parity")
	assert.Contains(t, applied.RequiredChecks, "go test ./...")
	assert.Equal(t, newSource, readWorkspaceFiles(t, projectDir, []string{sourceRel})[sourceRel])
	assert.Equal(t, "generated surface before\n", readWorkspaceFiles(t, projectDir, []string{generatedRel})[generatedRel])
}

func TestPromoteCandidate_RejectsSymlinkedADKSourcePath(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	outsideDir := t.TempDir()
	linkRel := "autopus-adk/content/skills/testing-strategy.md"
	linkPath := filepath.Join(projectDir, filepath.FromSlash(linkRel))
	require.NoError(t, os.MkdirAll(filepath.Dir(linkPath), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(outsideDir, "target.md"), linkPath))

	candidate := promotionReadyCandidate(validSkillContent("testing-strategy") + "new body\n")

	result, err := PromoteCandidate(context.Background(), PromotionOptions{
		ProjectDir: projectDir,
		Candidate:  candidate,
		Approval:   HumanApproval{ApprovedBy: "human-reviewer"},
		Apply:      true,
	})

	require.Error(t, err)
	assert.False(t, result.Applied)
	assert.NoFileExists(t, filepath.Join(outsideDir, "target.md"))
}

func TestArchiveCandidate_PreservesReasonProvenanceAndReplayEvidence(t *testing.T) {
	t.Parallel()

	quarantineDir := t.TempDir()
	candidate := promotionReadyCandidate(validSkillContent("testing-strategy") + "archived body\n")
	candidate.Status = "quarantined"

	result, err := ArchiveCandidate(context.Background(), ArchiveOptions{
		QuarantineDir: quarantineDir,
		Candidate:     candidate,
		Reason:        "stale",
	})

	require.NoError(t, err)
	assert.Equal(t, "archived", result.Status)
	assert.Equal(t, "stale", result.ReasonCode)
	assert.ElementsMatch(t, candidate.Provenance.SourceFailureRefs, result.Provenance.SourceFailureRefs)
	assert.ElementsMatch(t, candidate.ReplayEvidenceRefs, result.ReplayEvidenceRefs)
	assert.FileExists(t, result.ArchivePath)

	body, err := os.ReadFile(result.ArchivePath)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"reason_code": "stale"`)
	assert.Contains(t, string(body), "qamesh-run-1/run-index.json")
	assert.Contains(t, string(body), "qamesh-run-1/manifest.json#AC-QAMESH2-006")
}

func writeSkillEvolvePackageRunIndex(t *testing.T, dir, status string) string {
	t.Helper()
	manifest := `{"status":"passed","reproduction_command":"go test ./pkg/skillevolve -run Replay -count=1"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644))
	runIndex := `{"status":"` + status + `","manifest_paths":["manifest.json"],"checks":[{"id":"must-semantic-output","adapter":"go-test","status":"passed"}]}` + "\n"
	path := filepath.Join(dir, "run-index.json")
	require.NoError(t, os.WriteFile(path, []byte(runIndex), 0o644))
	return path
}

func promotionReadyCandidate(content string) CandidateBundle {
	return CandidateBundle{
		ID:             "cand-promotion-ready",
		Status:         "promotion_ready",
		PromotionReady: true,
		ProposedFiles: []ProposedFile{{
			Path:    "autopus-adk/content/skills/testing-strategy.md",
			Content: content,
		}},
		Provenance: CandidateProvenance{
			SourceFailureRefs:      []string{"qamesh-run-1/manifest.json#AC-QAMESH2-006"},
			SourceHashes:           []string{fakeSHA("a")},
			EvidenceRefs:           []string{"qamesh-run-1/run-index.json"},
			GenerationPromptDigest: fakeSHA("e"),
			RedactionStatus:        "passed",
			Creator:                "tester-agent",
			AffectedAcceptanceIDs:  []string{"AC-SEVOLVE-001", "AC-SEVOLVE-003"},
			AffectedSourceOfTruths: []string{"autopus-adk/content/skills/testing-strategy.md"},
			AffectedGeneratedSurfaces: []string{
				".codex/skills/testing-strategy.md",
			},
		},
		ReplayEvidenceRefs: []string{"qamesh-run-1/run-index.json", "qamesh-run-1/manifest.json"},
	}
}
