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
