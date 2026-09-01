package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteGUIExploreWritesManifestWithGUIArtifacts(t *testing.T) {
	dir := fixtureGUIProject(t)
	prependFakeNodeAndNPM(t, dir)
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})

	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	require.Len(t, result.ManifestPaths, 1)
	assert.Equal(t, "gui-explore", result.AdapterResults[0].Adapter)

	body, err := os.ReadFile(projectPath(dir, result.ManifestPaths[0]))
	require.NoError(t, err)
	manifest := string(body)
	assert.NotContains(t, manifest, dir)
	assert.NotContains(t, manifest, "/Users/")
	assert.Contains(t, manifest, `"surface": "frontend"`)
	assert.Contains(t, manifest, `"source_spec": "SPEC-QAMESH-003"`)
	assert.Contains(t, manifest, `"adapter": "gui-explore"`)
	assert.Contains(t, manifest, `"kind": "journey_graph"`)
	assert.Contains(t, manifest, `"kind": "aria_snapshot"`)
	assert.Contains(t, manifest, `"kind": "console_summary"`)
	assert.Contains(t, manifest, `"kind": "network_summary"`)
	assert.Contains(t, manifest, `"kind": "screenshot_quarantine_ref"`)
	assert.Contains(t, manifest, `"id": "gui-policy-runtime"`)
	assert.Contains(t, manifest, `"type": "gui_runtime_policy"`)
	assert.Contains(t, manifest, `"actual": "runtime_policy_enforced=true`)
}

func fixtureGUIProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	pack := map[string]any{
		"id":      "gui-smoke",
		"title":   "GUI smoke",
		"surface": "frontend",
		"lanes":   []string{"gui-explore"},
		"adapter": map[string]any{"id": "gui-explore"},
		"command": map[string]any{"run": "npm exec playwright test", "cwd": ".", "timeout": "60s"},
		"checks":  []map[string]any{{"id": "gui-smoke", "type": "gui_exploration"}},
		"artifacts": []map[string]any{
			{"kind": "journey_graph", "path": ".autopus/qa/gui/journey-graph.json"},
			{"kind": "aria_snapshot", "path": ".autopus/qa/gui/a11y.aria.yml"},
			{"kind": "console_summary", "path": ".autopus/qa/gui/console-summary.json"},
			{"kind": "network_summary", "path": ".autopus/qa/gui/network-summary.json"},
			{"kind": "screenshot_quarantine_ref", "path": ".autopus/qa/gui/screenshot-ref.json"},
		},
		"gui": map[string]any{
			"allowed_origins":   []string{"http://127.0.0.1:4173"},
			"forbidden_actions": []string{"mutation", "payment", "email_send"},
			"selector_strategy": "role-first",
			"network_policy":    map[string]any{"mode": "summary-only"},
			"artifact_retention": map[string]any{
				"publish_raw": false,
			},
		},
	}
	body, err := json.Marshal(pack)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "gui-smoke.yaml"), body, 0o644))
	return dir
}

func prependFakeNodeAndNPM(t *testing.T, projectDir string) {
	t.Helper()
	bin := filepath.Join(projectDir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "node"), []byte(fakeGuardReadyNode()), 0o755))
	npm := `#!/bin/sh
mkdir -p .autopus/qa/gui
[ -f "$AUTOPUS_QAMESH_GUI_POLICY_PATH" ] || exit 7
[ "$AUTOPUS_QAMESH_GUI_ALLOWED_ORIGINS" = "http://127.0.0.1:4173" ] || exit 8
[ "$AUTOPUS_QAMESH_GUI_FORBIDDEN_ACTIONS" = "mutation,payment,email_send" ] || exit 9
printf '{"runtime_policy_enforced":true,"allowed_origins":["http://127.0.0.1:4173"],"forbidden_actions":["mutation","payment","email_send"],"routes":["/"],"stopped_actions":[]}' > .autopus/qa/gui/journey-graph.json
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/account"}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
echo gui ok
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "npm"), []byte(npm), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakeGuardReadyNode() string {
	return `#!/bin/sh
if [ -n "$AUTOPUS_QAMESH_GUI_GUARD_READY_PATH" ]; then
  mkdir -p "$(dirname "$AUTOPUS_QAMESH_GUI_GUARD_READY_PATH")"
  printf '{"schema_version":"autopus.qamesh.gui_guard.v1","installed":true}' > "$AUTOPUS_QAMESH_GUI_GUARD_READY_PATH"
fi
exit 0
`
}
