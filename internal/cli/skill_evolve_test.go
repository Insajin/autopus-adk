package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/skillevolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillEvolveCmd_RegistersCandidatesJSONSmoke(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	for _, path := range [][]string{
		{"skill", "evolve"},
		{"skill", "evolve", "candidates"},
		{"skill", "evolve", "replay"},
		{"skill", "evolve", "promote"},
		{"skill", "evolve", "archive"},
	} {
		cmd, remaining, err := root.Find(path)
		require.NoError(t, err)
		require.NotNil(t, cmd)
		require.Empty(t, remaining)
		assert.Equal(t, "auto "+strings.Join(path, " "), cmd.CommandPath())
	}

	dir := t.TempDir()
	indexPath := writeSkillEvolveCLIQualityIndex(t, dir)
	quarantineDir := filepath.Join(dir, "quarantine")

	root = NewRootCmd()
	root.SetArgs([]string{
		"skill", "evolve", "candidates",
		"--quality-index", indexPath,
		"--quarantine", quarantineDir,
		"--min-count", "2",
		"--format", "json",
	})
	var out bytes.Buffer
	root.SetOut(&out)

	require.NoError(t, root.Execute())

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto skill evolve candidates")
	data := payload["data"].(map[string]any)
	candidates := data["candidates"].([]any)
	require.Len(t, candidates, 1)

	candidate := candidates[0].(map[string]any)
	assert.Equal(t, "oracle.structural_only.missing_semantic_output", candidate["fingerprint"])
	assert.Equal(t, "quarantined", candidate["status"])
	assert.Equal(t, false, candidate["active"])
	assert.NotEmpty(t, candidate["source_failures"])
	assert.NotEmpty(t, candidate["source_hashes"])
	assert.Contains(t, strings.TrimSpace(candidate["proposed_digest"].(string)), "sha256:")
	assert.Contains(t, candidate["affected_refs"], "autopus-adk/content/skills/testing-strategy.md")
	assert.Contains(t, candidate["affected_acceptance_ids"], "AC-SEVOLVE-001")
	assert.NotEmpty(t, candidate["replay_plan"])
	assert.NotEmpty(t, candidate["bundle_path"])
	assert.FileExists(t, candidate["bundle_path"].(string))
	proposedFiles := candidate["proposed_files"].([]any)
	require.NotEmpty(t, proposedFiles)
	assert.NotContains(t, proposedFiles[0].(map[string]any), "content")
}

func TestSkillEvolveCmd_ReplayPromoteArchiveJSONSmoke(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	quarantineDir := filepath.Join(projectDir, "quarantine")
	sourceRel := "autopus-adk/content/skills/testing-strategy.md"
	generatedRel := ".codex/skills/testing-strategy.md"
	oldSource := "---\nname: testing-strategy\ndescription: Old\n---\nold\n"
	newSource := "---\nname: testing-strategy\ndescription: Candidate skill improvement\n---\nnew\n"
	writeSkillEvolveWorkspaceFile(t, projectDir, sourceRel, oldSource)
	writeSkillEvolveWorkspaceFile(t, projectDir, generatedRel, "generated before\n")
	runIndexPath := writeSkillEvolveReplayFixture(t, projectDir)

	candidate := skillevolve.CandidateBundle{
		ID:         "cand-cli-replay",
		Status:     "quarantined",
		Active:     false,
		BundlePath: filepath.Join(quarantineDir, "cand-cli-replay.json"),
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
		ProposedFiles: []skillevolve.ProposedFile{{Path: sourceRel, Content: newSource}},
		Provenance: skillevolve.CandidateProvenance{
			SourceFailureRefs:     []string{"qamesh-run-1/manifest.json#AC-QAMESH2-006"},
			SourceHashes:          []string{"sha256:" + strings.Repeat("a", 64)},
			EvidenceRefs:          []string{runIndexPath},
			RedactionStatus:       "passed",
			Creator:               "tester-agent",
			AffectedAcceptanceIDs: []string{"AC-SEVOLVE-001", "AC-SEVOLVE-003"},
		},
		ReplayEvidenceRefs: []string{runIndexPath},
	}
	writeSkillEvolveCandidateBundle(t, candidate.BundlePath, candidate)

	replayOut := executeSkillEvolveCommand(t, []string{
		"skill", "evolve", "replay", candidate.ID,
		"--project-dir", projectDir,
		"--quarantine", quarantineDir,
		"--format", "json",
	})
	replayPayload := decodeJSONMap(t, replayOut)
	assertCommonJSONEnvelope(t, replayPayload, "auto skill evolve replay")
	assert.Equal(t, true, replayPayload["data"].(map[string]any)["promotion_ready"])

	promoteOut := executeSkillEvolveCommand(t, []string{
		"skill", "evolve", "promote", candidate.ID,
		"--project-dir", projectDir,
		"--quarantine", quarantineDir,
		"--approved-by", "human-reviewer",
		"--apply",
		"--format", "json",
	})
	promotePayload := decodeJSONMap(t, promoteOut)
	assertCommonJSONEnvelope(t, promotePayload, "auto skill evolve promote")
	assert.Equal(t, true, promotePayload["data"].(map[string]any)["applied"])
	assert.Equal(t, newSource, readSkillEvolveWorkspaceFile(t, projectDir, sourceRel))
	assert.Equal(t, "generated before\n", readSkillEvolveWorkspaceFile(t, projectDir, generatedRel))

	archiveOut := executeSkillEvolveCommand(t, []string{
		"skill", "evolve", "archive", candidate.ID,
		"--quarantine", quarantineDir,
		"--reason", "stale",
		"--format", "json",
	})
	archivePayload := decodeJSONMap(t, archiveOut)
	assertCommonJSONEnvelope(t, archivePayload, "auto skill evolve archive")
	archiveData := archivePayload["data"].(map[string]any)
	assert.Equal(t, "archived", archiveData["status"])
	assert.Equal(t, "stale", archiveData["reason_code"])
	assert.FileExists(t, archiveData["archive_path"].(string))
}

func TestSkillEvolveCmd_RejectsCandidateIDPathTraversal(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	root.SetArgs([]string{
		"skill", "evolve", "replay", "../outside",
		"--quarantine", t.TempDir(),
		"--format", "json",
	})
	var out bytes.Buffer
	root.SetOut(&out)

	require.Error(t, root.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "error", payload["status"])
	assert.Contains(t, payload["error"].(map[string]any)["message"], "invalid candidate id")
}

func writeSkillEvolveCLIQualityIndex(t *testing.T, dir string) string {
	t.Helper()

	payload := map[string]any{
		"schema_version": "autopus.quality_index.v1",
		"failures": []map[string]any{
			skillEvolveCLIFailure("qamesh-run-1/manifest.json#AC-QAMESH2-006", "a"),
			skillEvolveCLIFailure("qamesh-run-2/manifest.json#AC-QAMESH2-006", "b"),
			skillEvolveCLIFailure("learn/2026-05-06T02:00:00Z.jsonl#L17", "c"),
		},
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(dir, "quality-index.json")
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
	return path
}

func skillEvolveCLIFailure(ref, hashChar string) map[string]any {
	return map[string]any{
		"ref":          ref,
		"fingerprint":  "oracle.structural_only.missing_semantic_output",
		"source_hash":  "sha256:" + strings.Repeat(hashChar, 64),
		"evidence_ref": "autopus-adk/pkg/skillevolve/testdata/qamesh-run-1/run-index.json",
		"affected_refs": []string{
			"autopus-adk/content/skills/testing-strategy.md",
		},
		"acceptance_refs":  []string{"AC-QAMESH2-006", "AC-SEVOLVE-001"},
		"expected":         "oracle report includes concrete semantic output rows",
		"actual":           "oracle report only includes headings and section labels",
		"failure_severity": "must",
	}
}

func executeSkillEvolveCommand(t *testing.T, args []string) []byte {
	t.Helper()
	root := NewRootCmd()
	root.SetArgs(args)
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	return out.Bytes()
}

func writeSkillEvolveCandidateBundle(t *testing.T, path string, candidate skillevolve.CandidateBundle) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	body, err := json.MarshalIndent(candidate, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
}

func writeSkillEvolveReplayFixture(t *testing.T, dir string) string {
	t.Helper()
	runDir := filepath.Join(dir, "qamesh-run")
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	manifest := map[string]any{
		"status":               "passed",
		"reproduction_command": "go test ./pkg/skillevolve -run Replay -count=1",
	}
	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "manifest.json"), append(manifestBody, '\n'), 0o644))
	runIndex := map[string]any{
		"status":         "passed",
		"manifest_paths": []string{"manifest.json"},
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

func writeSkillEvolveWorkspaceFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func readSkillEvolveWorkspaceFile(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(body)
}
