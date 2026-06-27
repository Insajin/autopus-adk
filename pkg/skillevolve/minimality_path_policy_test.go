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

func TestMinimalityPathPolicy_BlocksGeneratedRuntimeAndRootArtifacts(t *testing.T) {
	t.Parallel()

	paths := []string{
		".codex/skills/auto-go.md",
		".claude/commands/auto-go.md",
		".gemini/skills/auto-go/SKILL.md",
		".opencode/rules/autopus/auto-go.md",
		".agents/skills/auto-go/SKILL.md",
		".autopus/plugins/index.json",
		".autopus/runtime/session.json",
		".autopus/orchestra/run.json",
		".autopus/brainstorms/BS-001.md",
		".autopus/canary/report.json",
		".autopus/context/signatures.md",
		".autopus/foo-manifest.json",
		"config.toml",
		".codex/plugins/cache/autopus-local/auto/1.0.0/skills/auto-go/SKILL.md",
		"autopus-adk/.codex/skills/auto-go.md",
		"autopus-adk/.agents/skills/auto-go/SKILL.md",
		"autopus-adk/.autopus/runtime/session.json",
		"autopus-adk/.autopus/foo-manifest.json",
		"autopus-adk/config.toml",
		"/abs/.codex/skills/auto-go.md",
		"../.codex/skills/auto-go.md",
		"..\\.codex\\skills\\auto-go.md",
		"autopus-adk/.autopus/context/signatures.md",
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			require.True(t, isGeneratedSurfacePath(path), "path policy must block %s", path)
			result, err := EvaluateSafety(context.Background(), CandidateBundle{
				ID: "cand-minimality-generated-path",
				ProposedFiles: []ProposedFile{{
					Path:    path,
					Content: validSkillContent("auto-go"),
				}},
			}, SafetyOptions{})
			require.NoError(t, err)
			assert.False(t, result.Allowed)
			assert.False(t, result.ReplayAllowed)
			assert.False(t, result.PromotionAllowed)
			assert.Contains(t, result.ReasonCodes, "generated_surface_mutation_forbidden")
		})
	}
}

func TestReplayCandidate_MinimalityGeneratedPathVariantsBlockPromotionReady(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"autopus-adk/.codex/skills/auto-go.md",
		"autopus-adk/.autopus/runtime/session.json",
		"../.codex/skills/auto-go.md",
		"/abs/.codex/skills/auto-go.md",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			result, err := ReplayCandidate(context.Background(), ReplayOptions{
				Candidate: CandidateBundle{
					ID:     "cand-minimality-replay-generated",
					Status: "quarantined",
					ProposedFiles: []ProposedFile{{
						Path:    path,
						Content: validSkillContent("auto-go"),
					}},
				},
			})

			require.NoError(t, err)
			assert.False(t, result.PromotionReady)
			assert.Contains(t, result.FailureReasons, "generated_surface_mutation_forbidden")
		})
	}
}

func TestGenerateCandidates_MinimalityDefaultThresholdCreatesInactiveQuarantineOnly(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	quarantineDir := filepath.Join(projectDir, ".autopus", "skill-evolve", "quarantine")
	result, err := GenerateCandidates(context.Background(), CandidateGenerationOptions{
		ProjectDir:       projectDir,
		QualityIndexPath: writeMinimalityQualityIndex(t, projectDir, []string{"autopus-adk/content/skills/agent-pipeline.md"}),
		QuarantineDir:    quarantineDir,
		Creator:          "tester-agent",
	})

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)

	candidate := result.Candidates[0]
	assert.Equal(t, "minimality.duplicate_helper.settings-form", candidate.Fingerprint)
	assert.Equal(t, "quarantined", candidate.Status)
	assert.False(t, candidate.Active)
	assert.False(t, candidate.PromotionReady)
	assert.Empty(t, candidate.SafetyReasonCodes)
	assertPathWithin(t, quarantineDir, candidate.BundlePath)
	assert.FileExists(t, candidate.BundlePath)
}

func TestGenerateCandidates_MinimalityGeneratedAffectedRefsRemainRejected(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	result, err := GenerateCandidates(context.Background(), CandidateGenerationOptions{
		ProjectDir: projectDir,
		QualityIndexPath: writeMinimalityQualityIndex(t, projectDir, []string{
			".codex/skills/auto-go.md",
			".agents/skills/auto-go/SKILL.md",
			".autopus/runtime/session.json",
			"config.toml",
		}),
		QuarantineDir: filepath.Join(projectDir, ".autopus", "skill-evolve", "quarantine"),
		Creator:       "tester-agent",
	})

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)

	candidate := result.Candidates[0]
	assert.Equal(t, "rejected", candidate.Status)
	assert.False(t, candidate.Active)
	assert.False(t, candidate.PromotionReady)
	assert.Contains(t, candidate.SafetyReasonCodes, "affected_file_outside_owned_paths")
}

func writeMinimalityQualityIndex(t *testing.T, dir string, affectedRefs []string) string {
	t.Helper()

	payload := map[string]any{
		"schema_version": "autopus.quality_index.v1",
		"failures": []qualityFailure{
			{
				Ref:             "qamesh-run-1/manifest.json#AC-MINDISC-012",
				Fingerprint:     "minimality.duplicate_helper.settings-form",
				SourceHash:      fakeSHA("d"),
				EvidenceRef:     "autopus-adk/pkg/skillevolve/testdata/qamesh-run-1/run-index.json",
				AffectedRefs:    affectedRefs,
				AcceptanceRefs:  []string{"AC-MINDISC-012"},
				Expected:        "reuse existing helper before adding a duplicate helper",
				Actual:          "duplicate helper proposed",
				FailureSeverity: "must",
			},
			{
				Ref:             "qamesh-run-2/manifest.json#AC-MINDISC-012",
				Fingerprint:     "minimality.duplicate_helper.settings-form",
				SourceHash:      fakeSHA("e"),
				EvidenceRef:     "autopus-adk/pkg/skillevolve/testdata/qamesh-run-1/run-index.json",
				AffectedRefs:    affectedRefs,
				AcceptanceRefs:  []string{"AC-MINDISC-012"},
				Expected:        "reuse existing helper before adding a duplicate helper",
				Actual:          "duplicate helper proposed again",
				FailureSeverity: "must",
			},
		},
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(dir, "minimality-quality-index.json")
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
	return path
}
