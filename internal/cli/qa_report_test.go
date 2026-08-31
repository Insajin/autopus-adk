package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qacapture "github.com/insajin/autopus-adk/pkg/qa/capture"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	qareport "github.com/insajin/autopus-adk/pkg/qa/report"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestQAReportCmd_WritesSelfContainedHTML(t *testing.T) {
	t.Parallel()

	dir := writeQAReportFixture(t)
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"report", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa report")
	assert.Equal(t, "warn", payload["status"])

	data := payload["data"].(map[string]any)
	assert.Equal(t, qareport.SchemaVersion, data["schema_version"])
	assert.Equal(t, "failed", data["verdict"])
	assert.Equal(t, "shareable", data["retention"])
	reportPath := data["report_path"].(string)
	assert.Equal(t, filepath.Join(dir, ".autopus", "qa", "runs", "qa-cli-1", "report.html"), reportPath)

	body, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	html := string(body)
	assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
	assert.Contains(t, html, "checkout-login")
	assert.Contains(t, html, "expired session stayed on checkout")
	assert.NotContains(t, html, "<script")
}

func TestQAReportCmd_TextOutputNamesReportPath(t *testing.T) {
	t.Parallel()

	dir := writeQAReportFixture(t)
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"report", "--project-dir", dir})

	require.NoError(t, cmd.Execute())
	text := out.String()
	assert.Contains(t, text, "qa report failed ingestion=complete journeys=1")
	assert.Contains(t, text, "retention=shareable")
	assert.Contains(t, text, "report: ")
	assert.Contains(t, text, "next: auto qa release --dry-run --format json")
}

func TestQAReportCmd_NoWriteSkipsFile(t *testing.T) {
	t.Parallel()

	dir := writeQAReportFixture(t)
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"report", "--project-dir", dir, "--no-write", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	_, hasPath := data["report_path"]
	assert.False(t, hasPath)
	_, err := os.Stat(filepath.Join(dir, ".autopus", "qa", "runs", "qa-cli-1", "report.html"))
	assert.True(t, os.IsNotExist(err))
}

func TestQAReportCmd_RejectsGeneratedSurfaceOutput(t *testing.T) {
	t.Parallel()

	dir := writeQAReportFixture(t)
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"report", "--project-dir", dir, "--output", ".claude/report.html"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "may not target generated surface .claude")
}

func TestQAReportCmd_FailsWithoutRunIndex(t *testing.T) {
	t.Parallel()

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"report", "--project-dir", t.TempDir(), "--format", "json"})

	err := cmd.Execute()
	require.Error(t, err)
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "error", payload["status"])
	errorPayload := payload["error"].(map[string]any)
	assert.Equal(t, "qa_report_failed", errorPayload["code"])
}

// TestQAReportNextCommand_OmitsDefaultProjectDir keeps the printed hint
// copy-pasteable: a redundant `--project-dir .` invites the reader to think the
// flag is required.
func TestQAReportNextCommand_OmitsDefaultProjectDir(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "auto qa report", qaReportNextCommand("."))
	assert.Equal(t, "auto qa report", qaReportNextCommand("  "))
	assert.Equal(t, "auto qa report --project-dir /tmp/shop", qaReportNextCommand("/tmp/shop"))
}

// TestQARunCmd_HintsReportOnlyWhenEvidenceExists pins the discoverability seam:
// the hint appears exactly when a run index was written — including on a failed
// run, which already wrote its index and is when the report matters most.
func TestQARunCmd_HintsReportOnlyWhenEvidenceExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))

	dryRun := newQACmd()
	var dryOut bytes.Buffer
	dryRun.SetOut(&dryOut)
	dryRun.SetArgs([]string{"run", "--project-dir", dir, "--output", filepath.Join(dir, "runs"), "--dry-run"})
	require.NoError(t, dryRun.Execute())
	assert.NotContains(t, dryOut.String(), "auto qa report")

	// A bare module has no buildable test target, so this run fails. The hint
	// must still appear: the index exists and the report explains the failure.
	failing := newQACmd()
	var failOut bytes.Buffer
	failing.SetOut(&failOut)
	failing.SetErr(&failOut)
	failing.SetArgs([]string{"run", "--project-dir", dir, "--output", filepath.Join(dir, "runs")})
	require.Error(t, failing.Execute())
	assert.Contains(t, failOut.String(), "next: auto qa report --project-dir "+dir)
}

func TestQAReportCmd_EmbedMediaInlinesLocalScreenshots(t *testing.T) {
	t.Parallel()

	dir := writeQAReportFixture(t)
	output := filepath.Join(t.TempDir(), "embedded.html")
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"report", "--project-dir", dir, "--embed-media", "--output", output})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "retention=local-only")

	body, err := os.ReadFile(output)
	require.NoError(t, err)
	html := string(body)
	assert.Contains(t, html, "data:image/png;base64,")
	assert.Contains(t, html, "do not publish, attach, or share this file")
	assert.Contains(t, html, "submit-expired")
	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "ZgotmplZ")
}

// writeQAReportFixture lays out one failed browser journey exactly as `auto qa
// run` would, so the CLI test exercises real ingestion rather than a stub.
func writeQAReportFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runDir := filepath.Join(dir, ".autopus", "qa", "runs", "qa-cli-1")
	manifestRel := filepath.ToSlash(filepath.Join(".autopus", "qa", "runs", "qa-cli-1", "checkout-login", "manifest.json"))

	artifactRel := filepath.ToSlash(filepath.Join("artifacts", "stdout", "stdout.log"))
	artifactPath := filepath.Join(runDir, "checkout-login", filepath.FromSlash(artifactRel))
	require.NoError(t, os.MkdirAll(filepath.Dir(artifactPath), 0o755))
	require.NoError(t, os.WriteFile(artifactPath, []byte("1 failing test\n"), 0o644))

	writeJSONFile(t, filepath.Join(runDir, "checkout-login", "manifest.json"), qaevidence.Manifest{
		SchemaVersion: qaevidence.SchemaVersionV2,
		QAResultID:    "qa-checkout-login",
		Surface:       "frontend",
		Lane:          "browser-staging",
		ScenarioRef:   "checkout-login",
		Runner:        qaevidence.Runner{Name: "playwright", Command: "npm exec playwright test"},
		Status:        "failed",
		StartedAt:     "2026-01-02T03:00:00Z",
		EndedAt:       "2026-01-02T03:00:15Z",
		DurationMS:    15000,
		Artifacts: []qaevidence.ArtifactRef{
			{Kind: "stdout", Path: artifactRel, Publishable: true, Redaction: "text_redacted_and_scanned"},
			writeQACaptureArtifact(t, filepath.Join(runDir, "checkout-login")),
		},
		OracleResults: qaevidence.OracleResults{Checks: []qaevidence.CheckResult{{
			ID: "login-rejects-expired-session", Type: "browser", Status: "failed",
			Expected: "expired session is redirected", Actual: "expired session stayed on checkout",
			FailureSummary: "expired sessions are not redirected before checkout",
		}}},
		RedactionStatus: qaevidence.RedactionStatus{Status: "passed"},
		SourceRefs: qaevidence.SourceRefs{
			SourceSpec: "SPEC-QAMESH-005", AcceptanceRefs: []string{"AC-001"},
			JourneyID: "checkout-login", StepID: "step-1", Adapter: "playwright",
		},
		RetentionClass:      "local-redacted-local-media",
		ReproductionCommand: "npm exec playwright test",
	})

	writeJSONFile(t, filepath.Join(runDir, "run-index.json"), qarun.Index{
		SchemaVersion:   qarun.RunIndexSchemaVersion,
		RunID:           "qa-cli-1",
		Workspace:       qarun.WorkspaceRef{WorkspaceID: "fixture", RepoID: "fixture-repo", RepoRoot: "."},
		Status:          "failed",
		StartedAt:       "2026-01-02T03:00:00Z",
		EndedAt:         "2026-01-02T03:00:15Z",
		Profile:         "local",
		Lane:            "browser-staging",
		ManifestPaths:   []string{manifestRel},
		AdapterResults:  []qarun.AdapterResult{{Adapter: "playwright", JourneyID: "checkout-login", Status: "failed", QAMESHManifestPath: manifestRel, RepairPromptAvailable: true}},
		RedactionStatus: qarun.RedactionStatus{Status: "passed"},
	})
	return dir
}

// writeQACaptureArtifact publishes a real sanitized capture index under the
// journey's artifacts directory and writes the PNG bytes it references into the
// run's local capture directory — the layout `auto qa run` actually produces, so
// --embed-media resolves media the same way it does on a real run.
func writeQACaptureArtifact(t *testing.T, journeyDir string) qaevidence.ArtifactRef {
	t.Helper()
	artifactDir := filepath.Join(journeyDir, "artifacts", qacapture.ArtifactKind)
	captureDir := qacapture.LocalCaptureDir(journeyDir)

	img := image.NewRGBA(image.Rect(0, 0, 6, 4))
	for x := range 6 {
		img.Set(x, 2, color.RGBA{R: 0xe5, G: 0x53, B: 0x4b, A: 0xff})
	}
	var body bytes.Buffer
	require.NoError(t, png.Encode(&body, img))
	shotPath := filepath.Join(captureDir, "screenshots", "step-1.png")
	require.NoError(t, os.MkdirAll(filepath.Dir(shotPath), 0o755))
	require.NoError(t, os.WriteFile(shotPath, body.Bytes(), 0o644))
	sum := sha256.Sum256(body.Bytes())

	index := qacapture.Index{
		SchemaVersion: qacapture.IndexSchemaVersion,
		JourneyID:     "checkout-login",
		Mode:          qacapture.ModeAlways,
		Streams:       []string{qacapture.StreamScreenshot, qacapture.StreamConsole},
		StartedAt:     "2026-01-02T03:00:00Z",
		EndedAt:       "2026-01-02T03:00:15Z",
		Steps: []qacapture.Step{{
			StepID: "submit-expired", Order: 1, Title: "Submit with an expired login",
			Status: qacapture.StatusFailed, StartedAt: "2026-01-02T03:00:00Z",
			EndedAt: "2026-01-02T03:00:15Z", DurationMS: 15000,
			FailureSummary: "expired login stayed on checkout",
			Actions:        []qacapture.Action{{API: "page.click", TargetRef: "role:button[name=Pay]", DurationMS: 120}},
			Screenshot: &qacapture.MediaRef{
				Ref: "screenshots/step-1.png", Digest: "sha256:" + hex.EncodeToString(sum[:]),
				Bytes: int64(body.Len()), Width: 6, Height: 4, Retention: qacapture.RetentionLocalOnly,
			},
			Console: &qacapture.ConsoleSummary{Errors: 1, Messages: []qacapture.ConsoleMessage{
				{Severity: qacapture.SeverityError, Text: "POST /orders failed with 500", SourceRef: "origin:0/checkout.js"},
			}},
		}},
	}
	index.Totals = qacapture.ComputeTotals(index)
	require.NoError(t, qacapture.Validate(index))
	writeJSONFile(t, filepath.Join(artifactDir, qacapture.PublishedFileName), index)

	return qaevidence.ArtifactRef{
		Kind:        qacapture.ArtifactKind,
		Path:        filepath.ToSlash(filepath.Join("artifacts", qacapture.ArtifactKind, qacapture.PublishedFileName)),
		Publishable: true,
		Redaction:   "text_redacted_and_scanned",
	}
}
