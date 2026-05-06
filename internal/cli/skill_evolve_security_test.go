package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/skillevolve"
	"github.com/stretchr/testify/assert"
)

func TestSkillEvolveCmd_ReplayUsesProjectDirBoundary(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	outsideDir := t.TempDir()
	quarantineDir := filepath.Join(projectDir, "quarantine")
	runIndexPath := writeSkillEvolveReplayFixture(t, outsideDir)
	candidate := skillevolve.CandidateBundle{
		ID:                    "cand-cli-outside-replay",
		Status:                "quarantined",
		SourceHashes:          []string{"sha256:" + strings.Repeat("a", 64)},
		AffectedRefs:          []string{"autopus-adk/content/skills/testing-strategy.md"},
		AffectedAcceptanceIDs: []string{"AC-SEVOLVE-003"},
		BundlePath:            filepath.Join(quarantineDir, "cand-cli-outside-replay.json"),
		ReplayPlan: skillevolve.ReplayPlan{
			RunIndexPath: runIndexPath,
			Commands: []skillevolve.ReplayCommand{{
				Command: "go test ./pkg/skillevolve -run Replay -count=1",
			}},
			MustChecks: []skillevolve.ReplayCheckRef{{
				ID:            "must-semantic-output",
				AcceptanceRef: "AC-SEVOLVE-003",
				Source:        "run-index.json",
			}},
		},
		ProposedFiles: []skillevolve.ProposedFile{{
			Path:    "autopus-adk/content/skills/testing-strategy.md",
			Content: "---\nname: testing-strategy\ndescription: Candidate skill improvement\n---\nnew\n",
		}},
		Provenance: skillevolve.CandidateProvenance{
			SourceHashes:          []string{"sha256:" + strings.Repeat("a", 64)},
			EvidenceRefs:          []string{runIndexPath},
			AffectedAcceptanceIDs: []string{"AC-SEVOLVE-003"},
		},
		ReplayEvidenceRefs: []string{runIndexPath},
	}
	writeSkillEvolveCandidateBundle(t, candidate.BundlePath, candidate)

	out := executeSkillEvolveCommand(t, []string{
		"skill", "evolve", "replay", candidate.ID,
		"--project-dir", projectDir,
		"--quarantine", quarantineDir,
		"--format", "json",
	})

	payload := decodeJSONMap(t, out)
	data := payload["data"].(map[string]any)
	assert.Equal(t, false, data["promotion_ready"])
	assert.Contains(t, data["failure_reasons"], "replay_run_index_outside_project")
}
