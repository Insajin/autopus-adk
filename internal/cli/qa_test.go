package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQACmd_IsRegisteredWithEvidenceAndFeedback(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()

	qa, _, err := root.Find([]string{"qa"})
	require.NoError(t, err)
	require.NotNil(t, qa)

	evidence, _, err := root.Find([]string{"qa", "evidence"})
	require.NoError(t, err)
	require.NotNil(t, evidence)

	feedback, _, err := root.Find([]string{"qa", "feedback"})
	require.NoError(t, err)
	require.NotNil(t, feedback)
}

func TestQAEvidenceCmd_WritesManifestJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeQAFixtureManifest(t, dir, "failed")
	output := filepath.Join(dir, "evidence")

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"evidence",
		"--surface", "browser",
		"--lane", "golden",
		"--scenario", "browser:login",
		"--input", input,
		"--output", output,
		"--format", "json",
	})

	require.NoError(t, cmd.Execute())

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa evidence")
	data := payload["data"].(map[string]any)
	assert.Equal(t, "qa-browser-login-001", data["qa_result_id"])
	assert.Equal(t, "failed", data["status"])
	assert.Equal(t, "passed", data["redaction_status"].(map[string]any)["status"])
	manifestPath := data["manifest_path"].(string)
	assert.FileExists(t, manifestPath)
	assert.Contains(t, manifestPath, output)
}

func TestQAFeedbackCmd_WritesProviderBundlesAndRejectsUnsupportedTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeQAFixtureManifest(t, dir, "failed")
	evidenceOut := filepath.Join(dir, "evidence")
	promptOut := filepath.Join(dir, "prompts")

	evidenceCmd := newQACmd()
	evidenceCmd.SetArgs([]string{"evidence", "--surface", "browser", "--lane", "golden", "--scenario", "browser:login", "--input", input, "--output", evidenceOut, "--format", "json"})
	var evidenceStdout bytes.Buffer
	evidenceCmd.SetOut(&evidenceStdout)
	require.NoError(t, evidenceCmd.Execute())
	manifestPath := decodeJSONMap(t, evidenceStdout.Bytes())["data"].(map[string]any)["manifest_path"].(string)

	for _, target := range []string{"codex", "claude", "gemini", "opencode"} {
		target := target
		t.Run(target, func(t *testing.T) {
			cmd := newQACmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"feedback", "--to", target, "--evidence", manifestPath, "--output", promptOut, "--format", "json"})

			require.NoError(t, cmd.Execute())
			data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
			bundlePath := data["prompt_bundle_path"].(string)
			assert.FileExists(t, filepath.Join(bundlePath, "repair-prompt.md"))
			body, err := os.ReadFile(filepath.Join(bundlePath, "repair-prompt.md"))
			require.NoError(t, err)
			assert.Contains(t, string(body), "Untrusted deterministic QA evidence")
			assert.Contains(t, string(body), "AC-QAMESH-001")
		})
	}

	unsupportedOut := filepath.Join(dir, "unsupported")
	cmd := newQACmd()
	cmd.SetArgs([]string{"feedback", "--to", "unsupported", "--evidence", manifestPath, "--output", unsupportedOut, "--format", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.Error(t, cmd.Execute())
	assert.NoDirExists(t, unsupportedOut)
}

func TestQAEvidenceCmd_RedactsTextInputBeforeManifestPublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeQAFixtureManifest(t, dir, "failed")
	artifactPath := filepath.Join(dir, "artifacts", "console.json")
	require.NoError(t, os.WriteFile(artifactPath, []byte("Authorization: Bearer sk-proj-qameshfake1234567890"), 0o644))

	output := filepath.Join(dir, "evidence")
	cmd := newQACmd()
	cmd.SetArgs([]string{"evidence", "--surface", "browser", "--lane", "golden", "--scenario", "browser:login", "--input", input, "--output", output, "--format", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	artifactBody, err := os.ReadFile(filepath.Join(output, "artifacts", "console", "console.json"))
	require.NoError(t, err)
	assert.Contains(t, string(artifactBody), "[REDACTED_SECRET]")
	assert.NotContains(t, string(artifactBody), "sk-proj-qameshfake1234567890")
}

func TestQAEvidenceCmd_RejectsBinaryArtifactWithoutManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeQAFixtureManifest(t, dir, "failed")
	raw, err := os.ReadFile(input)
	require.NoError(t, err)
	raw = bytes.ReplaceAll(raw, []byte("console.json"), []byte("trace.zip"))
	require.NoError(t, os.WriteFile(input, raw, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "artifacts", "trace.zip"), []byte("raw trace bytes"), 0o644))

	output := filepath.Join(dir, "evidence")
	cmd := newQACmd()
	cmd.SetArgs([]string{"evidence", "--surface", "browser", "--lane", "golden", "--scenario", "browser:login", "--input", input, "--output", output, "--format", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.Error(t, cmd.Execute())
	assert.NoDirExists(t, output)
}

func TestQAEvidenceCmd_ReportsNormalizedA11yFailureStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeQAFixtureManifest(t, dir, "passed")
	raw, err := os.ReadFile(input)
	require.NoError(t, err)
	raw = bytes.Replace(raw, []byte(`"critical_count": 0`), []byte(`"critical_count": 1`), 1)
	require.NoError(t, os.WriteFile(input, raw, 0o644))

	output := filepath.Join(dir, "evidence")
	cmd := newQACmd()
	cmd.SetArgs([]string{"evidence", "--surface", "browser", "--lane", "golden", "--scenario", "browser:login", "--input", input, "--output", output, "--format", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "failed", data["status"])
	assert.Equal(t, true, data["repair_prompt_available"])
}

func writeQAFixtureManifest(t *testing.T, dir, status string) string {
	t.Helper()
	artifactDir := filepath.Join(dir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))
	consolePath := filepath.Join(artifactDir, "console.json")
	require.NoError(t, os.WriteFile(consolePath, []byte(`{"messages":["redacted console"]}`+"\n"), 0o644))

	manifest := map[string]any{
		"schema_version":       "qamesh.evidence.v1",
		"qa_result_id":         "qa-browser-login-001",
		"surface":              "browser",
		"lane":                 "golden",
		"scenario_ref":         "browser:login",
		"runner":               map[string]any{"name": "playwright", "command": "npx playwright test e2e/tests/qamesh-golden.spec.ts --project=chromium"},
		"status":               status,
		"started_at":           "2026-05-02T00:00:00Z",
		"ended_at":             "2026-05-02T00:00:01Z",
		"duration_ms":          1000,
		"repair_prompt_ref":    "",
		"retention_class":      "local-redacted",
		"reproduction_command": "PLAYWRIGHT_SKIP_GLOBAL_SETUP=true npx playwright test e2e/tests/qamesh-golden.spec.ts --project=chromium",
		"artifacts": []map[string]any{{
			"kind":        "console",
			"path":        consolePath,
			"publishable": true,
			"redaction":   "text_redacted_and_scanned",
		}},
		"oracle_results": map[string]any{
			"a11y": map[string]any{
				"critical_count": 0,
				"serious_count":  0,
				"failed_targets": []string{},
			},
		},
		"redaction_status": map[string]any{"status": "passed", "findings": []string{}},
		"source_refs": map[string]any{
			"source_spec":         "SPEC-QAMESH-001",
			"acceptance_refs":     []string{"AC-QAMESH-001", "AC-QAMESH-003"},
			"owned_paths":         []string{"Autopus/frontend/e2e/tests/qamesh-golden.spec.ts"},
			"do_not_modify_paths": []string{".codex/**", ".opencode/**", ".autopus/plugins/**"},
		},
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	input := filepath.Join(dir, "input.json")
	require.NoError(t, os.WriteFile(input, append(body, '\n'), 0o644))
	return input
}
