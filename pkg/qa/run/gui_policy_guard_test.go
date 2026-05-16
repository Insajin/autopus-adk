package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGUIPolicyRuntimeBlocksStoppedActions(t *testing.T) {
	dir := fixtureGUIProject(t)
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
cat > .autopus/qa/gui/journey-graph.json <<'JSON'
{
  "runtime_policy_enforced": true,
  "stopped_actions": [
    {
      "reason": "off_origin_navigation",
      "attempted_url": "https://evil.example/phish",
      "allowed_origins": ["http://127.0.0.1:4173"],
      "side_effect": "blocked_before_navigation"
    },
    {
      "reason": "forbidden_action",
      "action_class": "mutation",
      "target": "role=button[name='Save billing settings']",
      "side_effect": "blocked_before_dispatch"
    }
  ]
}
JSON
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/account"}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
exit 0
`)

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiPolicyRuntimeCheckID)
	require.Len(t, result.ManifestPaths, 1)
	manifest := loadManifest(t, result.ManifestPaths[0])
	assert.Equal(t, "blocked", manifest.Status)
	check := manifestCheck(t, manifest, guiPolicyRuntimeCheckID)
	assert.Equal(t, guiPolicyRuntimeCheckType, check.Type)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Expected, "allowed_origins=http://127.0.0.1:4173")
	assert.Contains(t, check.Expected, "forbidden_actions=mutation,payment,email_send")
	assert.Contains(t, check.Actual, "off_origin_navigation:https://evil.example/phish")
	assert.Contains(t, check.Actual, "forbidden_action:mutation")
	graph := readFinalArtifact(t, result.ManifestPaths[0], manifest, "journey_graph")
	assert.Contains(t, graph, `"side_effect": "blocked_before_navigation"`)
	assert.Contains(t, graph, `"side_effect": "blocked_before_dispatch"`)
}

func TestGUIPolicyRuntimeBlocksMissingEnforcementConfirmation(t *testing.T) {
	dir := fixtureGUIProject(t)
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
printf '{"routes":["/"],"stopped_actions":[]}' > .autopus/qa/gui/journey-graph.json
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/account"}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
exit 0
`)

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiPolicyRuntimeCheckID)
	require.Len(t, result.ManifestPaths, 1)
	manifest := loadManifest(t, result.ManifestPaths[0])
	check := manifestCheck(t, manifest, guiPolicyRuntimeCheckID)
	assert.Equal(t, "blocked", manifest.Status)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Actual, "runtime_policy_enforced=false")
	assert.Contains(t, check.Actual, "missing=journey_graph.runtime_policy_enforced")
}

func TestGUIPolicyRuntimeBlocksUnavailableGUITarget(t *testing.T) {
	dir := fixtureGUIProject(t)
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
printf '{"runtime_policy_enforced":true,"availability":{"status":"unavailable","reason":"page.goto: net::ERR_CONNECTION_REFUSED at http://127.0.0.1:4173/"},"stopped_actions":[]}' > .autopus/qa/gui/journey-graph.json
printf '{"available":false,"fallback_reason":"base_url_unavailable"}' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/","status_code":null}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
exit 0
`)
	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})
	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiPolicyRuntimeCheckID)
	require.Len(t, result.ManifestPaths, 1)
	manifest := loadManifest(t, result.ManifestPaths[0])
	check := manifestCheck(t, manifest, guiPolicyRuntimeCheckID)
	assert.Equal(t, "blocked", manifest.Status)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Actual, "target_availability=unavailable")
	assert.Contains(t, check.FailureSummary, "unavailable")
}

func TestGUIPolicyRuntimeBlocksBeforeCommandWhenGuardPreflightFails(t *testing.T) {
	dir := fixtureGUIProject(t)
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	npm := `#!/bin/sh
mkdir -p .autopus/qa/gui
printf 'unsafe side effect' > .autopus/qa/gui/side-effect.txt
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "npm"), []byte(npm), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiPolicyRuntimeCheckID)
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "qa", "gui", "side-effect.txt"))
}

func TestGUIPolicyRuntimeBlocksMismatchedSchemeNetworkSummary(t *testing.T) {
	dir := fixtureGUIProject(t)
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
printf '{"runtime_policy_enforced":true,"routes":["/"],"stopped_actions":[]}' > .autopus/qa/gui/journey-graph.json
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"https://127.0.0.1:4173/account"}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
exit 0
`)

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiPolicyRuntimeCheckID)
	manifest := loadManifest(t, result.ManifestPaths[0])
	check := manifestCheck(t, manifest, guiPolicyRuntimeCheckID)
	assert.Contains(t, check.Actual, "network_request_outside_allowed:https://127.0.0.1:4173/account")
}

func TestGUIPolicyNetworkOriginNormalizesDefaultPorts(t *testing.T) {
	doc := map[string]any{"requests": []any{
		map[string]any{"url": "https://example.test/account"},
		map[string]any{"url": "http://example.test/home"},
	}}

	outside := outsideAllowedNetworkRequests(doc, []string{"https://example.test:443", "http://example.test:80"})

	assert.Empty(t, outside)
}

func TestGUIArtifactRedactionBlocksUnsafeDeclaredArtifact(t *testing.T) {
	dir := fixtureGUIProject(t)
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
cat > .autopus/qa/gui/journey-graph.json <<'JSON'
{
  "runtime_policy_enforced": true,
  "routes": ["/"],
  "stopped_actions": [],
  "authorization": "Bearer sk-proj-unsafe000000000000",
  "local_path": "/Users/alice/private/notes.md"
}
JSON
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/account"}]}' > .autopus/qa/gui/network-summary.json
printf '{"sha256":"abc123","local_only":true}' > .autopus/qa/gui/screenshot-ref.json
exit 0
`)

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiArtifactPublicationCheckID)
	require.Len(t, result.ManifestPaths, 1)
	body, err := os.ReadFile(result.ManifestPaths[0])
	require.NoError(t, err)
	assert.NotContains(t, string(body), "sk-proj-unsafe000000000000")
	assert.NotContains(t, string(body), "/Users/alice")
	assert.NotContains(t, string(body), `"kind": "journey_graph"`)

	manifest := loadManifest(t, result.ManifestPaths[0])
	check := manifestCheck(t, manifest, guiArtifactPublicationCheckID)
	assert.Equal(t, guiArtifactPublicationCheckType, check.Type)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Actual, "sensitive_json_value:journey_graph")
	assert.Contains(t, check.Actual, "local_user_path:journey_graph")
}

func TestGUIArtifactRedactionBlocksRawAndMissingArtifacts(t *testing.T) {
	dir := fixtureGUIProject(t)
	rewriteGUIArtifacts(t, dir, []map[string]any{
		{"kind": "journey_graph", "path": ".autopus/qa/gui/journey-graph.json"},
		{"kind": "aria_snapshot", "path": ".autopus/qa/gui/a11y.aria.yml"},
		{"kind": "console_summary", "path": ".autopus/qa/gui/console-summary.json"},
		{"kind": "network_summary", "path": ".autopus/qa/gui/network-summary.json"},
		{"kind": "screenshot", "path": ".autopus/qa/gui/raw-screenshot.png"},
		{"kind": "trace", "path": ".autopus/qa/gui/missing-trace.zip"},
	})
	prependGUICommand(t, dir, `#!/bin/sh
mkdir -p .autopus/qa/gui
printf '{"runtime_policy_enforced":true,"routes":["/"],"stopped_actions":[]}' > .autopus/qa/gui/journey-graph.json
printf -- '- main\n' > .autopus/qa/gui/a11y.aria.yml
printf '{"messages":[]}' > .autopus/qa/gui/console-summary.json
printf '{"requests":[{"url":"http://127.0.0.1:4173/account"}]}' > .autopus/qa/gui/network-summary.json
printf 'raw bytes' > .autopus/qa/gui/raw-screenshot.png
exit 0
`)

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Contains(t, result.FailedChecks, guiArtifactPublicationCheckID)
	manifest := loadManifest(t, result.ManifestPaths[0])
	check := manifestCheck(t, manifest, guiArtifactPublicationCheckID)
	assert.Contains(t, check.Actual, "raw_media_artifact:screenshot")
	assert.Contains(t, check.Actual, "raw_media_artifact:trace")
	assert.Contains(t, check.Actual, "artifact_unreadable:trace")
	assert.NotContains(t, string(mustReadFile(t, result.ManifestPaths[0])), "raw-screenshot.png")
}

func prependGUICommand(t *testing.T, projectDir, npm string) {
	t.Helper()
	bin := filepath.Join(projectDir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "node"), []byte(fakeGuardReadyNode()), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "npm"), []byte(npm), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func rewriteGUIArtifacts(t *testing.T, projectDir string, artifacts []map[string]any) {
	t.Helper()
	journeyPath := filepath.Join(projectDir, ".autopus", "qa", "journeys", "gui-smoke.yaml")
	raw, err := os.ReadFile(journeyPath)
	require.NoError(t, err)
	var pack map[string]any
	require.NoError(t, json.Unmarshal(raw, &pack))
	pack["artifacts"] = artifacts
	body, err := json.Marshal(pack)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(journeyPath, body, 0o644))
}

func loadManifest(t *testing.T, path string) qaevidence.Manifest {
	t.Helper()
	manifest, err := qaevidence.LoadManifest(path)
	require.NoError(t, err)
	return manifest
}

func manifestCheck(t *testing.T, manifest qaevidence.Manifest, id string) qaevidence.CheckResult {
	t.Helper()
	for _, check := range manifest.OracleResults.Checks {
		if check.ID == id {
			return check
		}
	}
	require.FailNow(t, "missing manifest check", id)
	return qaevidence.CheckResult{}
}

func readFinalArtifact(t *testing.T, manifestPath string, manifest qaevidence.Manifest, kind string) string {
	t.Helper()
	for _, artifact := range manifest.Artifacts {
		if artifact.Kind == kind {
			body, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), artifact.Path))
			require.NoError(t, err)
			return string(body)
		}
	}
	require.FailNow(t, "missing artifact", kind)
	return ""
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return body
}
