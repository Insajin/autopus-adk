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

func TestQAPlanCmd_ReportsDetectedAdapters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js"}}`), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"plan", "--project-dir", dir, "--lane", "fast", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Contains(t, stringSlice(data["detected_adapters"]), "go-test")
	assert.Contains(t, stringSlice(data["detected_adapters"]), "node-script")
	assert.NotEmpty(t, data["run_index_preview_path"])
}

func TestQARunCmd_DryRunDoesNotWriteRunIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))
	output := filepath.Join(dir, "runs")

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run", "--project-dir", dir, "--output", output, "--dry-run", "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "ok", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, true, data["dry_run"])
	assert.Empty(t, data["run_index_path"])
	assert.NoDirExists(t, output)
}

func TestQAExploreCmd_DryRunSelectsGUIAdapter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := []byte(`id: gui-smoke
title: GUI smoke
surface: frontend
lanes: [gui-explore]
adapter:
  id: gui-explore
command:
  run: npm exec playwright test
  cwd: .
  timeout: 60s
checks:
  - id: gui
    type: gui_exploration
artifacts:
  - kind: journey_graph
    path: .autopus/qa/gui/journey-graph.json
  - kind: network_summary
    path: .autopus/qa/gui/network-summary.json
gui:
  allowed_origins: ["http://127.0.0.1:4173"]
  forbidden_actions: ["mutation", "payment", "email_send"]
  selector_strategy: role-first
  network_policy:
    mode: summary-only
  artifact_retention:
    publish_raw: false
`)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "gui-smoke.yaml"), body, 0o644))
	output := filepath.Join(dir, "runs")

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"explore", "--project-dir", dir, "--output", output, "--dry-run", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, true, data["dry_run"])
	assert.Contains(t, stringSlice(data["selected_adapters"]), "gui-explore")
	assert.Contains(t, stringSlice(data["selected_journeys"]), "gui-smoke")
	assert.NotEmpty(t, data["run_index_preview_path"])
	assert.NotEmpty(t, data["manifest_output_preview_paths"])
	artifactRefs, ok := data["artifact_preview_refs"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, artifactRefs)
	assert.Contains(t, artifactRefs[0].(map[string]any)["path"].(string), ".autopus/qa/gui/")
	assert.NoDirExists(t, output)
}

func TestQAExploreCmd_DryRunReportsProjectLocalJourneyGap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"explore", "--project-dir", dir, "--dry-run", "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "warn", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, "warning", data["status"])
	contract := data["harness_contract"].(map[string]any)
	assert.Equal(t, "harness", contract["role"])
	assert.Equal(t, "project-local", contract["journey_pack_ownership"])
	gaps := data["setup_gaps"].([]any)
	require.NotEmpty(t, gaps)
	assert.Contains(t, gaps[0].(map[string]any)["reason"].(string), "ADK is a harness")
	assert.NoDirExists(t, filepath.Join(dir, ".autopus", "qa", "runs"))
}

func TestQACommandsRejectGeneratedSurfaceOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeQAFixtureManifest(t, dir, "failed")

	evidenceCmd := newQACmd()
	evidenceCmd.SetArgs([]string{"evidence", "--surface", "browser", "--lane", "golden", "--scenario", "browser:login", "--input", input, "--output", filepath.Join(dir, ".codex", "qa"), "--format", "json"})
	require.Error(t, evidenceCmd.Execute())

	feedbackCmd := newQACmd()
	feedbackCmd.SetArgs([]string{"feedback", "--to", "codex", "--evidence", input, "--output", filepath.Join(dir, ".opencode", "qa"), "--format", "json"})
	require.Error(t, feedbackCmd.Execute())

	runCmd := newQACmd()
	runCmd.SetArgs([]string{"run", "--project-dir", dir, "--output", filepath.Join(dir, "nested", ".codex", "qa"), "--dry-run", "--format", "json"})
	require.Error(t, runCmd.Execute())

	mixedRunCmd := newQACmd()
	mixedRunCmd.SetArgs([]string{"run", "--project-dir", dir, "--output", filepath.Join(dir, ".CODEX", "qa"), "--dry-run", "--format", "json"})
	require.Error(t, mixedRunCmd.Execute())
}

func stringSlice(value any) []string {
	raw, _ := json.Marshal(value)
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}
