package qualityloop

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFailures_RepeatedMinimalityFindingsCreateInactiveSkillCandidates(t *testing.T) {
	t.Parallel()

	var inputs []FailureInput
	for _, reason := range minimalityReasonCodes() {
		for index := 1; index <= 3; index++ {
			inputs = append(inputs, minimalityFailureInput(reason, "settings-form", index))
		}
		for index := 1; index <= 2; index++ {
			inputs = append(inputs, minimalityFailureInput(reason, "one-off", index))
		}
	}

	result, err := NormalizeFailures(inputs)
	require.NoError(t, err)

	repeated := make(map[string]ImprovementCandidate)
	for _, candidate := range result.Candidates {
		if !contains(candidate.ReasonCodes, "repeated_failure") {
			assert.False(t, candidate.ApplyEnabled)
			assert.NotContains(t, candidate.FailureFingerprint, ".settings-form")
			continue
		}
		repeated[candidate.FailureFingerprint] = candidate
	}
	require.Len(t, repeated, len(minimalityReasonCodes()))

	for _, reason := range minimalityReasonCodes() {
		fingerprint := "minimality." + reason + ".settings-form"
		candidate, ok := repeated[fingerprint]
		require.True(t, ok, "missing repeated candidate for %s", reason)
		assert.Equal(t, KindSkillEvolveCandidate, candidate.CandidateKind)
		assert.Equal(t, StatusQuarantined, candidate.Status)
		assert.False(t, candidate.Active)
		assert.False(t, candidate.ApplyEnabled)
		assert.False(t, candidate.RepairActionEnabled)
		assert.Equal(t, RedactionMetadataOnly, candidate.RedactionStatus)
		assert.Contains(t, candidate.ReasonCodes, "repeated_failure")
		assert.Contains(t, candidate.ReasonCodes, reason)
		assert.Len(t, candidate.SourceFailureRefs, 3)
		assert.Equal(t, "autopus-adk", candidate.OwningRepo)
	}
}

func TestNormalizeFailures_MinimalityGeneratedTargetsDoNotBecomeActiveRepeatedCandidates(t *testing.T) {
	t.Parallel()

	targets := []string{
		".codex/skills/auto-go.md",
		".claude/commands/auto-go.md",
		".gemini/skills/auto-go/SKILL.md",
		".opencode/rules/autopus/auto-go.md",
		".agents/skills/auto-go/SKILL.md",
		".codex/plugins/cache/autopus-local/auto/1.0.0/skills/auto-go/SKILL.md",
		".autopus/runtime/session.json",
		".autopus/orchestra/run.json",
		".autopus/brainstorms/BS-001.md",
		".autopus/canary/report.json",
		".autopus/context/signatures.md",
		".autopus/foo-manifest.json",
		"config.toml",
	}

	for _, target := range targets {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			inputs := []FailureInput{
				minimalityGeneratedTargetInput(target, 1),
				minimalityGeneratedTargetInput(target, 2),
				minimalityGeneratedTargetInput(target, 3),
			}
			result, err := NormalizeFailures(inputs)
			require.NoError(t, err)
			require.Len(t, result.Candidates, 3)

			for _, candidate := range result.Candidates {
				assert.NotContains(t, candidate.ReasonCodes, "repeated_failure")
				assert.False(t, candidate.Active)
				assert.False(t, candidate.ApplyEnabled)
				assert.False(t, candidate.RepairActionEnabled)
				assert.Contains(t, candidate.ReasonCodes, "generated_surface_mutation_forbidden")
				assert.Contains(t, candidate.SafetyReasonCodes, "generated_surface_mutation_forbidden")
			}
		})
	}
}

func TestNormalizeFailures_MinimalityTraversalRefsDoNotBecomeRepeatedCandidates(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"pkg/../../outside.md",
		"templates/../../../x",
		"autopus-adk/pkg/../../x",
	} {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			inputs := []FailureInput{
				minimalityTraversalInput(target, 1),
				minimalityTraversalInput(target, 2),
				minimalityTraversalInput(target, 3),
			}
			result, err := NormalizeFailures(inputs)
			require.NoError(t, err)
			require.Len(t, result.Candidates, 3)
			for _, candidate := range result.Candidates {
				assert.NotContains(t, candidate.ReasonCodes, "repeated_failure")
				assert.False(t, candidate.Active)
				assert.False(t, candidate.ApplyEnabled)
				assert.False(t, candidate.RepairActionEnabled)
			}
		})
	}
}

func minimalityFailureInput(reason, target string, index int) FailureInput {
	return FailureInput{
		SourceArtifactType:    "review",
		SourceID:              fmt.Sprintf("%s-%d", target, index),
		WorkspaceID:           "ws-quality",
		FailureFingerprint:    "minimality." + reason + "." + target,
		ReasonCode:            reason,
		DeterministicEvidence: true,
		RedactionStatus:       RedactionMetadataOnly,
		SourceHashes:          []string{fmt.Sprintf("sha256:%064d", index)},
		AffectedRefs:          []string{"pkg/adapter/codex/codex_extended_skill_rewrites_pipeline_completion.go"},
		AffectedAcceptanceIDs: []string{"AC-MINDISC-009"},
		TargetArtifact:        "pkg/adapter/codex/codex_extended_skill_rewrites_pipeline_completion.go",
	}
}

func minimalityGeneratedTargetInput(target string, index int) FailureInput {
	input := minimalityFailureInput("duplicate_helper", "generated-target", index)
	input.SourceID = fmt.Sprintf("generated-%d-%s", index, target)
	input.FailureFingerprint = "minimality.duplicate_helper.generated-target"
	input.TargetArtifact = target
	input.AffectedRefs = []string{target}
	return input
}

func minimalityTraversalInput(target string, index int) FailureInput {
	input := minimalityFailureInput("duplicate_helper", "traversal-target", index)
	input.SourceID = fmt.Sprintf("traversal-%d-%s", index, target)
	input.FailureFingerprint = "minimality.duplicate_helper.traversal-target"
	input.TargetArtifact = target
	input.AffectedRefs = []string{target}
	return input
}
