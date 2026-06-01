package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGUIPolicyScreenMatrixBlocksMissingScreenAndAction(t *testing.T) {
	dir := fixtureGUIProject(t)
	rewriteGUIPolicy(t, dir, map[string]any{
		"screen_matrix": []map[string]any{
			{"id": "dm", "path": "/dm", "required_actions": []string{"open-thread"}},
			{"id": "settings", "path": "/settings", "required_actions": []string{"open-channel"}},
		},
	})
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
cat > .autopus/qa/gui/journey-graph.json <<'JSON'
{
  "runtime_policy_enforced": true,
  "screens": [{"id":"dm","path":"/dm"}],
  "interactions": [{"screen_id":"dm","action_id":"open-thread","status":"passed"}],
  "stopped_actions": []
}
JSON
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/dm"}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
exit 0
`)

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiPolicyRuntimeCheckID)
	manifest := loadManifest(t, result.ManifestPaths[0])
	check := manifestCheck(t, manifest, guiPolicyRuntimeCheckID)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Actual, "missing_screens=settings")
	assert.Contains(t, check.Actual, "missing_screen_actions=settings:open-channel")
	assert.Contains(t, check.FailureSummary, "screen matrix")
}

func rewriteGUIPolicy(t *testing.T, projectDir string, updates map[string]any) {
	t.Helper()
	journeyPath := filepath.Join(projectDir, ".autopus", "qa", "journeys", "gui-smoke.yaml")
	raw, err := os.ReadFile(journeyPath)
	require.NoError(t, err)
	var pack map[string]any
	require.NoError(t, json.Unmarshal(raw, &pack))
	gui, ok := pack["gui"].(map[string]any)
	require.True(t, ok)
	for key, value := range updates {
		gui[key] = value
	}
	body, err := json.Marshal(pack)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(journeyPath, body, 0o644))
}
