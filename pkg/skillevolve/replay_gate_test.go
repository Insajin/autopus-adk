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

func TestReplayCandidate_BlocksStaticSafetyFailures(t *testing.T) {
	t.Parallel()

	result, err := ReplayCandidate(context.Background(), ReplayOptions{
		Candidate: CandidateBundle{
			ID:     "cand-generated-surface",
			Status: "quarantined",
			ProposedFiles: []ProposedFile{{
				Path:    ".codex/skills/testing-strategy.md",
				Content: validSkillContent("testing-strategy"),
			}},
		},
	})

	require.NoError(t, err)
	assert.False(t, result.PromotionReady)
	assert.Contains(t, result.FailureReasons, "generated_surface_mutation_forbidden")
}

func TestReplayCandidate_RequiresConcreteOutputAndAcceptanceMapping(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	runIndexPath := writeReplayGateRun(t, projectDir, replayGateRunOptions{})
	t.Run("missing affected output", func(t *testing.T) {
		t.Parallel()

		candidate := replayGateCandidate(runIndexPath)
		candidate.AffectedRefs = nil
		candidate.ProposedFiles = nil
		candidate.Provenance.AffectedSourceOfTruths = nil

		result, err := ReplayCandidate(context.Background(), ReplayOptions{
			ProjectDir: projectDir,
			Candidate:  candidate,
		})

		require.NoError(t, err)
		assert.False(t, result.PromotionReady)
		assert.Contains(t, result.FailureReasons, "missing_acceptance_mapping")
	})
	t.Run("missing acceptance refs", func(t *testing.T) {
		t.Parallel()

		candidate := replayGateCandidate(runIndexPath)
		candidate.AffectedAcceptanceIDs = nil
		candidate.ReplayPlan.AcceptanceRefs = nil
		candidate.ReplayPlan.MustChecks[0].AcceptanceRef = ""
		candidate.Provenance.AffectedAcceptanceIDs = nil

		result, err := ReplayCandidate(context.Background(), ReplayOptions{
			ProjectDir: projectDir,
			Candidate:  candidate,
		})

		require.NoError(t, err)
		assert.False(t, result.PromotionReady)
		assert.Contains(t, result.FailureReasons, "missing_acceptance_mapping")
	})
}

func TestReplayCandidate_RecordsNonPassedRunIndex(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	runIndexPath := writeReplayGateRun(t, projectDir, replayGateRunOptions{RunStatus: "failed"})

	result, err := ReplayCandidate(context.Background(), ReplayOptions{
		ProjectDir: projectDir,
		Candidate:  replayGateCandidate(runIndexPath),
	})

	require.NoError(t, err)
	assert.False(t, result.PromotionReady)
	assert.Contains(t, result.FailureReasons, "replay_run_index_not_passed:failed")
}

func TestReplayCandidate_RejectsUnboundRunIndexEvidence(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	runIndexPath := writeReplayGateRun(t, projectDir, replayGateRunOptions{})
	candidate := replayGateCandidate(runIndexPath)
	candidate.Provenance.EvidenceRefs = nil
	candidate.ReplayEvidenceRefs = nil
	candidate.SourceFailures = nil

	result, err := ReplayCandidate(context.Background(), ReplayOptions{
		ProjectDir: projectDir,
		Candidate:  candidate,
	})

	require.NoError(t, err)
	assert.False(t, result.PromotionReady)
	assert.Contains(t, result.FailureReasons, "replay_evidence_not_in_provenance")
}

func TestReplayCandidate_RequiresCandidateReplayCommand(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	runIndexPath := writeReplayGateRun(t, projectDir, replayGateRunOptions{})
	candidate := replayGateCandidate(runIndexPath)
	candidate.ReplayPlan.Commands = nil

	result, err := ReplayCandidate(context.Background(), ReplayOptions{
		ProjectDir: projectDir,
		Candidate:  candidate,
	})

	require.NoError(t, err)
	assert.False(t, result.PromotionReady)
	assert.Contains(t, result.FailureReasons, "replay_command_missing")
	assert.Empty(t, result.Evidence.Commands)
}

func TestReplayCandidate_RejectsManifestEvidenceFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		run        replayGateRunOptions
		wantReason string
	}{
		{
			name:       "manifest path escapes run directory",
			run:        replayGateRunOptions{ManifestPath: "../outside/manifest.json"},
			wantReason: "replay_manifest_path_invalid",
		},
		{
			name:       "manifest status not passed",
			run:        replayGateRunOptions{ManifestStatus: "failed"},
			wantReason: "replay_manifest_not_passed",
		},
		{
			name:       "manifest command mismatches replay plan",
			run:        replayGateRunOptions{Command: "go test ./pkg/skillevolve -run Other -count=1"},
			wantReason: "replay_command_mismatch",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			runIndexPath := writeReplayGateRun(t, projectDir, tt.run)
			result, err := ReplayCandidate(context.Background(), ReplayOptions{
				ProjectDir: projectDir,
				Candidate:  replayGateCandidate(runIndexPath),
			})

			require.NoError(t, err)
			assert.False(t, result.PromotionReady)
			assert.Contains(t, result.FailureReasons, tt.wantReason)
		})
	}
}

type replayGateRunOptions struct {
	RunStatus      string
	ManifestPath   string
	ManifestStatus string
	Command        string
}

func replayGateCandidate(runIndexPath string) CandidateBundle {
	command := "go test ./pkg/skillevolve -run Replay -count=1"
	return CandidateBundle{
		ID:                    "cand-replay-gate",
		Status:                "quarantined",
		SourceHashes:          []string{fakeSHA("a")},
		AffectedRefs:          []string{"autopus-adk/content/skills/testing-strategy.md"},
		AffectedAcceptanceIDs: []string{"AC-SEVOLVE-003"},
		ReplayPlan: ReplayPlan{
			RunIndexPath: runIndexPath,
			Commands:     []ReplayCommand{{Command: command}},
			MustChecks: []ReplayCheckRef{{
				ID:            "must-semantic-output",
				AcceptanceRef: "AC-SEVOLVE-003",
				Source:        "run-index.json",
			}},
			AcceptanceRefs: []string{"AC-SEVOLVE-003"},
		},
		ProposedFiles: []ProposedFile{{
			Path:    "autopus-adk/content/skills/testing-strategy.md",
			Content: validSkillContent("testing-strategy") + "Replay gate update.\n",
		}},
		Provenance: CandidateProvenance{
			SourceHashes:              []string{fakeSHA("a")},
			EvidenceRefs:              []string{runIndexPath},
			AffectedAcceptanceIDs:     []string{"AC-SEVOLVE-003"},
			AffectedSourceOfTruths:    []string{"autopus-adk/content/skills/testing-strategy.md"},
			GenerationPromptDigest:    fakeSHA("b"),
			RedactionStatus:           "passed",
			AffectedGeneratedSurfaces: nil,
			SourceFailureRefs:         []string{"qamesh-run/manifest.json#AC-SEVOLVE-003"},
			Creator:                   "tester-agent",
		},
		ReplayEvidenceRefs: []string{runIndexPath},
	}
}

func writeReplayGateRun(t *testing.T, projectDir string, opts replayGateRunOptions) string {
	t.Helper()
	runDir := filepath.Join(projectDir, "qamesh-run")
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	if opts.RunStatus == "" {
		opts.RunStatus = "passed"
	}
	if opts.ManifestPath == "" {
		opts.ManifestPath = "manifest.json"
	}
	if opts.ManifestStatus == "" {
		opts.ManifestStatus = "passed"
	}
	if opts.Command == "" {
		opts.Command = "go test ./pkg/skillevolve -run Replay -count=1"
	}
	if opts.ManifestPath == "manifest.json" {
		manifest := map[string]any{
			"status":               opts.ManifestStatus,
			"reproduction_command": opts.Command,
		}
		body, err := json.MarshalIndent(manifest, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(runDir, "manifest.json"), append(body, '\n'), 0o644))
	}
	runIndex := map[string]any{
		"status":         opts.RunStatus,
		"manifest_paths": []string{opts.ManifestPath},
		"checks": []map[string]any{{
			"id":      "must-semantic-output",
			"adapter": "go-test",
			"status":  "passed",
		}},
	}
	body, err := json.MarshalIndent(runIndex, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(runDir, "run-index.json")
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
	return path
}
