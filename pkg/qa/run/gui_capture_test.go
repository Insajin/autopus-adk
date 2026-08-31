package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

func TestExecuteGUICapturePublishesTypedCaptureIndex(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript(""))
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})

	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	require.Len(t, result.ManifestPaths, 1)

	manifest := loadManifest(t, result.ManifestPaths[0])
	assert.Equal(t, RetentionLocalMedia, manifest.RetentionClass,
		"a journey that left raw media on disk must say so in its retention class")

	check := manifestCheck(t, manifest, guiCaptureCheckID)
	assert.Equal(t, "passed", check.Status)
	assert.Equal(t, guiCaptureCheckType, check.Type)
	assert.Contains(t, check.Actual, "steps=1")
	assert.Contains(t, check.Actual, "screenshots=1")

	// The sanitized projection is published; raw media never is.
	kinds := map[string]qaevidence.ArtifactRef{}
	for _, artifact := range manifest.Artifacts {
		kinds[artifact.Kind] = artifact
	}
	published, ok := kinds[capture.ArtifactKind]
	require.True(t, ok, "capture_index artifact must be published")
	assert.True(t, published.Publishable)
	for _, forbidden := range []string{"screenshot", "trace", "video", "har"} {
		assert.NotContains(t, kinds, forbidden)
	}

	body, err := os.ReadFile(filepath.Join(filepath.Dir(result.ManifestPaths[0]), published.Path))
	require.NoError(t, err)
	index, err := capture.DecodeIndex(body)
	require.NoError(t, err)
	require.NoError(t, capture.Validate(index))
	require.Len(t, index.Steps, 1)
	assert.Equal(t, "open-home", index.Steps[0].StepID)
	assert.Equal(t, capture.RetentionLocalOnly, index.Steps[0].Screenshot.Retention)
	assert.NotContains(t, string(body), dir, "published evidence must not carry local paths")
}

func TestExecuteGUICaptureBlocksWhenProducerWritesNoIndex(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript("skip-index"))
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})
	// Execute surfaces a blocked run as an error; the manifest is still written so
	// the failure is inspectable evidence rather than a lost run.
	require.EqualError(t, err, "qa run blocked")
	assert.Equal(t, "blocked", result.Status)
	require.Len(t, result.ManifestPaths, 1)
	check := manifestCheck(t, loadManifest(t, result.ManifestPaths[0]), guiCaptureCheckID)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.FailureSummary, "did not write capture-index.json")
}

func TestExecuteGUICaptureBlocksOnTotalsDrift(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript("bad-totals"))
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})
	require.EqualError(t, err, "qa run blocked")
	assert.Equal(t, "blocked", result.Status)
	check := manifestCheck(t, loadManifest(t, result.ManifestPaths[0]), guiCaptureCheckID)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.FailureSummary, "totals disagree with steps")
}

// TestExecuteGUICaptureBlocksOffPolicyOrigin proves the capture index is now the
// authoritative network evidence for the runtime policy oracle: an origin index
// outside the allowlist blocks the journey.
func TestExecuteGUICaptureBlocksOffPolicyOrigin(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript("off-origin"))
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})
	require.EqualError(t, err, "qa run blocked")
	assert.Equal(t, "blocked", result.Status)
	check := manifestCheck(t, loadManifest(t, result.ManifestPaths[0]), guiPolicyRuntimeCheckID)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Actual, "origin_out_of_policy")
	assert.Contains(t, check.FailureSummary, "off-origin network request")
}

// TestExecuteGUICaptureBlocksWithoutGuardReceipt is the anti-self-certification
// gate: a producer can write a perfect capture index, but only the harness-owned
// guard preload can witness that policy was enforced.
func TestExecuteGUICaptureBlocksWithoutGuardReceipt(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript("no-receipt"))
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})

	require.EqualError(t, err, "qa run blocked")
	assert.Equal(t, "blocked", result.Status)
	check := manifestCheck(t, loadManifest(t, result.ManifestPaths[0]), guiPolicyRuntimeCheckID)
	assert.Equal(t, "blocked", check.Status)
	assert.Contains(t, check.Actual, "missing=gui_policy_guard.receipt")
}
func TestExecuteGUICaptureBlocksWhenDeclaredMediaIsMissing(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript("drop-trace"))
	output := filepath.Join(dir, "runs")

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: output})
	require.EqualError(t, err, "qa run blocked")
	assert.Equal(t, "blocked", result.Status)
	check := manifestCheck(t, loadManifest(t, result.ManifestPaths[0]), guiCaptureCheckID)
	assert.Contains(t, check.FailureSummary, "is missing from the capture directory")
}

func fixtureGUICaptureProject(t *testing.T, capturePolicy map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	if capturePolicy == nil {
		capturePolicy = map[string]any{
			"mode":             "always",
			"streams":          []string{"screenshot", "console", "network", "trace"},
			"screenshot":       "on-failure",
			"console_severity": "warning",
			"retain_local":     true,
			"replay_script":    "optional",
		}
	}
	pack := map[string]any{
		"id":      "gui-capture-smoke",
		"title":   "GUI capture smoke",
		"surface": "frontend",
		"lanes":   []string{"gui-explore"},
		"adapter": map[string]any{"id": "gui-explore"},
		"command": map[string]any{"run": "npm exec playwright test", "cwd": ".", "timeout": "60s"},
		"checks":  []map[string]any{{"id": "gui-capture-smoke", "type": "gui_exploration"}},
		// Capture-enabled packs declare no artifacts: the harness emits the
		// capture_index itself, and enforcement evidence comes from the
		// guard-owned receipt rather than a producer-authored journey_graph.
		"gui": map[string]any{
			"allowed_origins":    []string{"http://127.0.0.1:4173"},
			"forbidden_actions":  []string{"mutation", "payment", "email_send"},
			"selector_strategy":  "role-first",
			"network_policy":     map[string]any{"mode": "local-only"},
			"artifact_retention": map[string]any{"publish_raw": false},
			"capture":            capturePolicy,
		},
	}
	body, err := json.Marshal(pack)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "gui-capture-smoke.yaml"), body, 0o644))
	return dir
}

func prependFakeCaptureProducer(t *testing.T, projectDir, npmScript string) {
	t.Helper()
	bin := filepath.Join(projectDir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "node"), []byte(fakeGuardReadyNode()), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "npm"), []byte(npmScript), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
