package scaffold

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestDesktopGUIExplorePackExampleDeclaresTypedCapturePolicy asserts the desktop
// example validates and declares the typed capture contract rather than the legacy
// magic artifact kinds. It declares no artifacts at all: the harness emits
// capture_index and the guard receipt witnesses enforcement, so nothing is
// producer-certified.
func TestDesktopGUIExplorePackExampleDeclaresTypedCapturePolicy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := desktopGUIExplorePackExample(projectSignals{PackageManager: "npm"})

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(body), &pack))
	require.NoError(t, journey.Validate(pack, dir))

	assert.Equal(t, DesktopGUIJourneyID, pack.ID)
	assert.Equal(t, "desktop", pack.Surface)
	assert.Equal(t, expectedCapturePolicy(), pack.GUI.Capture)

	assert.Empty(t, pack.Artifacts)
	for _, legacy := range []string{"journey_graph", "aria_snapshot", "console_summary", "network_summary", "screenshot_quarantine_ref"} {
		assert.NotContains(t, body, legacy)
	}
	assert.Contains(t, body, ".autopus/qa/runs/**")
}

// TestInitEmitsCaptureProducerAssets asserts a Playwright project gets all three
// producer assets, and that a pre-existing asset is preserved verbatim.
func TestInitEmitsCaptureProducerAssets(t *testing.T) {
	t.Parallel()
	dir := playwrightProject(t)

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	for _, id := range []string{"gui-capture-fixture", "gui-capture-reporter", "gui-capture-readme"} {
		assertCreatedID(t, result, id)
	}
	for _, name := range []string{"autopus-capture.fixture.cjs", "autopus-capture.reporter.cjs", "README.md"} {
		assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "capture", name))
	}
	readme := readString(t, filepath.Join(dir, ".autopus", "qa", "capture", "README.md"))
	assert.Contains(t, readme, "autopus-capture.reporter.cjs")
	assert.Contains(t, readme, "AUTOPUS_QAMESH_GUI_CAPTURE_INDEX_PATH")
}

func TestInitPreservesExistingCaptureProducerAsset(t *testing.T) {
	t.Parallel()
	dir := playwrightProject(t)
	target := filepath.Join(dir, ".autopus", "qa", "capture", "autopus-capture.fixture.cjs")
	writeFile(t, target, "// project owned\n")

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	assertSkippedID(t, result, "gui-capture-fixture")
	assertCreatedID(t, result, "gui-capture-reporter")
	assert.Equal(t, "// project owned\n", readString(t, target))
}

// TestCaptureProducerAssetsParseAsJavaScript guards the embedded asset bodies:
// a Go-side edit that breaks JavaScript syntax would only surface inside a real
// Playwright run otherwise.
func TestCaptureProducerAssetsParseAsJavaScript(t *testing.T) {
	t.Parallel()
	node := requireNode(t)
	dir := playwrightProject(t)
	_, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	for _, name := range []string{"autopus-capture.fixture.cjs", "autopus-capture.reporter.cjs"} {
		path := filepath.Join(dir, ".autopus", "qa", "capture", name)
		output, err := exec.Command(node, "--check", path).CombinedOutput()
		require.NoErrorf(t, err, "node --check %s: %s", name, output)
	}
}

// TestCaptureReporterEmitsIndexTheGoContractAccepts runs the emitted reporter over
// a synthetic shard and gates the result through every harness-side check. The
// producer and pkg/qa/capture agree only if all of them pass, so this is the test
// that would catch a totals or url_ref drift before a real GUI run does.
func TestCaptureReporterEmitsIndexTheGoContractAccepts(t *testing.T) {
	t.Parallel()
	node := requireNode(t)
	dir := playwrightProject(t)
	_, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(desktopGUIExplorePackExample(projectSignals{PackageManager: "npm"})), &pack))
	captureDir := filepath.Join(dir, ".autopus", "qa", "runs", "local", "capture")
	policyPath := writeCapturePolicy(t, captureDir, pack)
	seedCaptureShard(t, dir, captureDir)

	command := exec.Command(node, "-e", captureReporterDriver)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"AUTOPUS_TEST_REPORTER="+filepath.Join(dir, ".autopus", "qa", "capture", "autopus-capture.reporter.cjs"),
		"AUTOPUS_TEST_ROOT="+dir,
		"AUTOPUS_QAMESH_GUI_CAPTURE_POLICY_PATH="+policyPath,
		"AUTOPUS_QAMESH_GUI_CAPTURE_DIR="+captureDir,
		"AUTOPUS_QAMESH_GUI_CAPTURE_INDEX_PATH="+filepath.Join(captureDir, capture.IndexFileName),
	)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "capture reporter: %s", output)

	index, err := capture.LoadIndex(filepath.Join(captureDir, capture.IndexFileName))
	require.NoError(t, err)
	require.NoError(t, capture.Validate(index))
	assert.Empty(t, capture.Conform(index, pack.GUI.Capture))
	assert.Empty(t, capture.VerifyLocalMedia(index, captureDir))
	require.NoError(t, capture.AssertPublishable(capture.Sanitize(index)))

	require.Len(t, index.Steps, 1)
	assert.Equal(t, 1, index.Steps[0].Order)
	require.NotNil(t, index.Replay)
	assert.Equal(t, []string{"e2e/login.spec.ts"}, index.Replay.SpecRefs)
	assert.Equal(t, []string{"npm", "exec", "playwright", "test", "--", "e2e/login.spec.ts"}, index.Replay.Command)
	assert.Equal(t, capture.ComputeTotals(index), index.Totals)
}

const captureReporterDriver = `
const Reporter = require(process.env.AUTOPUS_TEST_REPORTER);
const reporter = new Reporter({});
reporter.onBegin({ rootDir: process.env.AUTOPUS_TEST_ROOT }, {});
reporter.onEnd({ status: 'passed' });
`

func playwrightProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"@playwright/test":"^1.0.0"}}`)
	writeFile(t, filepath.Join(dir, "playwright.config.ts"), "export default {}\n")
	return realPath(t, dir)
}

func requireNode(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; skipping JavaScript checks")
	}
	return node
}

func readString(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(body)
}

// writeCapturePolicy mirrors the effective policy document pkg/qa/run hands the
// producer, so the reporter is exercised against the real runtime handoff rather
// than a hand-tuned fixture.
func writeCapturePolicy(t *testing.T, captureDir string, pack journey.Pack) string {
	t.Helper()
	policy := pack.GUI.Capture
	body, err := json.Marshal(map[string]any{
		"schema_version":   "autopus.qamesh.gui_capture_policy.v1",
		"journey_id":       pack.ID,
		"allowed_origins":  pack.GUI.AllowedOrigins,
		"mode":             policy.Mode,
		"streams":          policy.Streams,
		"screenshot":       policy.Screenshot,
		"console_severity": policy.ConsoleSeverity,
		"retain_local":     policy.RetainLocal,
		"replay_script":    policy.ReplayScript,
	})
	require.NoError(t, err)
	path := filepath.Join(captureDir, "gui-capture-policy.json")
	writeFile(t, path, string(body))
	return path
}

func seedCaptureShard(t *testing.T, projectDir, captureDir string) {
	t.Helper()
	const stepID = "e2e-login-spec-ts-login-0-0-a1b2c3d4"
	writeFile(t, filepath.Join(projectDir, "e2e", "login.spec.ts"), "export default {};\n")
	writeFile(t, filepath.Join(captureDir, "traces", stepID+".zip"), "trace-bytes")
	shot := []byte("png-bytes")
	writeFile(t, filepath.Join(captureDir, "screenshots", stepID+".png"), string(shot))
	sum := sha256.Sum256(shot)
	body, err := json.Marshal(map[string]any{
		"step_id":     stepID,
		"title":       "login shows the dashboard",
		"status":      capture.StatusPassed,
		"started_at":  "2026-08-31T10:00:00.000Z",
		"ended_at":    "2026-08-31T10:00:04.000Z",
		"duration_ms": 4000,
		"spec_ref":    "e2e/login.spec.ts",
		"actions": []map[string]any{
			{"api": "goto", "target_ref": "origin:0/login", "duration_ms": 120},
			{"api": "fill", "target_ref": "#password", "duration_ms": 12},
		},
		"console": map[string]any{
			"errors": 1, "warnings": 2, "infos": 7,
			"messages": []map[string]any{{"severity": capture.SeverityError, "text": "render failed", "source_ref": "origin:0/src/app.js"}},
		},
		"network": map[string]any{
			"requests": 3, "failures": 1,
			"entries": []map[string]any{{"method": "GET", "url_ref": "origin:0/api/session", "status": 401, "resource_type": "xhr", "duration_ms": 33, "bytes": 12}},
		},
		"screenshot": map[string]any{
			"ref": "screenshots/" + stepID + ".png", "digest": "sha256:" + hex.EncodeToString(sum[:]),
			"bytes": len(shot), "width": 1280, "height": 720, "retention": capture.RetentionLocalOnly,
		},
	})
	require.NoError(t, err)
	writeFile(t, filepath.Join(captureDir, "shards", stepID+".json"), string(body))
}
