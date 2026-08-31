package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

const fixtureCaptureRunID = "qa-capture-1"

// captureFixture is a project laid out exactly as a real run leaves it: the
// sanitized index is published under the journey's artifacts directory while the
// raw media it references stays in <runDir>/_raw/<journey>/capture. Every digest
// is the true digest of the bytes on disk, so the embedding gates are exercised
// against files rather than stubs.
type captureFixture struct {
	fixture
	captureDir string   // local capture directory the media refs are relative to
	indexPath  string   // published capture index artifact
	shotPaths  []string // raw step screenshots, in step order
	decoyPaths []string // same-named files beside the published artifact
	index      capture.Index
}

func (f captureFixture) embedOptions() Options {
	opts := f.options()
	opts.EmbedMedia = true
	return opts
}

// rewriteIndex overwrites the published index, which is how a test simulates a
// producer that shipped a broken contract. Sanitize returns a deep copy of the
// step evidence, so a mutation here cannot leak into the fixture on disk.
func (f captureFixture) rewriteIndex(t *testing.T, mutate func(*capture.Index)) {
	t.Helper()
	index := capture.Sanitize(f.index)
	mutate(&index)
	writeJSON(t, f.indexPath, index)
}

func newCaptureFixture(t *testing.T) captureFixture {
	t.Helper()
	root := t.TempDir()
	runDir := filepath.Join(root, ".autopus", "qa", "runs", fixtureCaptureRunID)
	journeyDir := filepath.Join(runDir, "gui-capture")
	artifactDir := filepath.Join(journeyDir, "artifacts", capture.ArtifactKind)
	captureDir := capture.LocalCaptureDir(journeyDir)

	shots := make([]capture.MediaRef, 0, 2)
	paths := make([]string, 0, 2)
	decoys := make([]string, 0, 2)
	for order := 1; order <= 2; order++ {
		body := tinyPNG(t, 8, 4+order)
		rel := fmt.Sprintf("screenshots/step-%d.png", order)
		path := filepath.Join(captureDir, filepath.FromSlash(rel))
		writeBinary(t, path, body)
		paths = append(paths, path)
		// A same-named decoy of identical length sits beside the published
		// artifact. Resolving media there instead of in the capture directory
		// must fail the digest gate, which pins the resolution root.
		decoy := filepath.Join(artifactDir, filepath.FromSlash(rel))
		writeBinary(t, decoy, bytes.Repeat([]byte{0x7f}, len(body)))
		decoys = append(decoys, decoy)
		shots = append(shots, capture.MediaRef{
			Ref: rel, Digest: pngDigest(body), Bytes: int64(len(body)),
			Width: 8, Height: 4 + order, Retention: capture.RetentionLocalOnly,
		})
	}
	trace := []byte("PK\x03\x04 fixture trace bundle")
	writeBinary(t, filepath.Join(captureDir, "traces", "trace.zip"), trace)
	traceRef := capture.MediaRef{
		Ref: "traces/trace.zip", Digest: pngDigest(trace),
		Bytes: int64(len(trace)), Retention: capture.RetentionLocalOnly,
	}

	// Sanitize is what the harness persists, and it recomputes Totals, so the
	// fixture is exactly the artifact a conforming producer run leaves behind.
	index := capture.Sanitize(captureIndexFixture(shots, traceRef))
	require.NoError(t, capture.Validate(index))
	require.NoError(t, capture.AssertPublishable(index))
	body, err := json.MarshalIndent(index, "", "  ")
	require.NoError(t, err)

	writeManifest(t, root, runDir, manifestSpec{
		journey:        "gui-capture",
		surface:        "frontend",
		lane:           "gui-explore",
		adapter:        "gui-explore",
		status:         "failed",
		startedAt:      "2026-01-02T03:10:00Z",
		endedAt:        "2026-01-02T03:10:12Z",
		retentionClass: "local-redacted-local-media",
		check: evidence.CheckResult{
			ID: "checkout-redirects", Type: "browser", Status: "failed",
			Expected: "expired session redirected", Actual: "stayed on checkout",
			FailureSummary: "expired sessions are not redirected",
		},
		artifacts: []artifactSpec{
			{kind: "stdout", name: "stdout.log", body: "1 failing spec\n", publishable: true, redaction: "text_redacted_and_scanned"},
			{kind: capture.ArtifactKind, name: capture.PublishedFileName, body: string(body), publishable: true, redaction: "text_redacted_and_scanned"},
		},
	})

	runIndexPath := filepath.Join(runDir, RunIndexFile)
	writeJSON(t, runIndexPath, qarun.Index{
		SchemaVersion:   qarun.RunIndexSchemaVersion,
		RunID:           fixtureCaptureRunID,
		Workspace:       qarun.WorkspaceRef{WorkspaceID: "fixture", RepoID: "fixture-repo", RepoRoot: "."},
		Status:          "failed",
		StartedAt:       "2026-01-02T03:10:00Z",
		EndedAt:         "2026-01-02T03:10:12Z",
		Profile:         "local",
		Lane:            "gui-explore",
		ManifestPaths:   []string{filepath.ToSlash(filepath.Join(".autopus", "qa", "runs", fixtureCaptureRunID, "gui-capture", "manifest.json"))},
		AdapterResults:  []qarun.AdapterResult{{Adapter: "gui-explore", JourneyID: "gui-capture", Status: "failed", QAMESHManifestPath: filepath.ToSlash(filepath.Join(".autopus", "qa", "runs", fixtureCaptureRunID, "gui-capture", "manifest.json"))}},
		RedactionStatus: qarun.RedactionStatus{Status: "passed"},
	})
	return captureFixture{
		fixture:    fixture{root: root, runIndexPath: runIndexPath, runDir: runDir},
		captureDir: captureDir,
		indexPath:  filepath.Join(artifactDir, capture.PublishedFileName),
		shotPaths:  paths,
		decoyPaths: decoys,
		index:      index,
	}
}

// captureIndexFixture mirrors a two-step checkout journey: one passing step with
// a console warning and one failing step with a console error and a 500.
func captureIndexFixture(shots []capture.MediaRef, trace capture.MediaRef) capture.Index {
	first := shots[0]
	second := shots[1]
	index := capture.Index{
		SchemaVersion: capture.IndexSchemaVersion,
		JourneyID:     "gui-capture",
		Mode:          capture.ModeAlways,
		Streams:       []string{capture.StreamScreenshot, capture.StreamConsole, capture.StreamNetwork, capture.StreamTrace},
		StartedAt:     "2026-01-02T03:10:00Z",
		EndedAt:       "2026-01-02T03:10:12Z",
		Steps: []capture.Step{
			{
				StepID: "open-checkout", Order: 1, Title: "Open checkout", Status: capture.StatusPassed,
				StartedAt: "2026-01-02T03:10:00Z", EndedAt: "2026-01-02T03:10:03Z", DurationMS: 3000,
				ScreenRef: "screen:checkout",
				Actions:   []capture.Action{{API: "page.goto", TargetRef: "origin:0/checkout", DurationMS: 820}},
				Console: &capture.ConsoleSummary{Warnings: 1, Infos: 2, Messages: []capture.ConsoleMessage{
					{Severity: capture.SeverityWarning, Text: "hydration took 1200ms", SourceRef: "origin:0/app.js"},
				}},
				Network: &capture.NetworkSummary{Requests: 3, Entries: []capture.NetworkEntry{
					{Method: "GET", URLRef: "/checkout", Status: 200, ResourceType: "document", DurationMS: 118, Bytes: 4096},
				}},
				Screenshot: &first,
			},
			{
				StepID: "submit-expired", Order: 2, Title: "Submit with an expired login", Status: capture.StatusFailed,
				StartedAt: "2026-01-02T03:10:03Z", EndedAt: "2026-01-02T03:10:12Z", DurationMS: 9000,
				FailureSummary: "expired login stayed on checkout instead of redirecting",
				Actions: []capture.Action{
					{API: "page.getByRole", TargetRef: "role:button[name=Pay]", DurationMS: 42},
					{API: "page.click", TargetRef: "role:button[name=Pay]", DurationMS: 310},
				},
				Console: &capture.ConsoleSummary{Errors: 2, Messages: []capture.ConsoleMessage{
					{Severity: capture.SeverityError, Text: "POST /orders failed with 500", SourceRef: "origin:0/checkout.js"},
				}},
				Network: &capture.NetworkSummary{Requests: 4, Failures: 1, Entries: []capture.NetworkEntry{
					{Method: "POST", URLRef: "/orders", Status: 500, ResourceType: "xhr", DurationMS: 640, Bytes: 512},
				}},
				Screenshot: &second,
			},
		},
		Media: []capture.Media{{MediaRef: trace, Kind: capture.StreamTrace, StepID: "submit-expired"}},
		Replay: &capture.Replay{
			Kind:      capture.ReplayKindPlaywrightGrep,
			Command:   []string{"npm", "exec", "playwright", "test", "--grep", "checkout expired login"},
			SpecRefs:  []string{"tests/e2e/checkout.spec.ts"},
			Digest:    pngDigest([]byte("tests/e2e/checkout.spec.ts")),
			StepCount: 2,
		},
	}
	index.Totals = capture.ComputeTotals(index)
	return index
}

// tinyPNG encodes a real PNG so the embedding path reads decodable image bytes.
func tinyPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		img.Set(x, height/2, color.RGBA{R: 0x2f, G: 0x81, B: 0xf7, A: 0xff})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// pngDigest computes the digest independently of the production helper, so a
// wrong digest format in the projection cannot be masked by a shared bug.
func pngDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// corruptShot rewrites one byte in place: same length, different content, so the
// declared byte count still matches and only the digest can catch the swap.
func corruptShot(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Greater(t, len(body), 8)
	body[len(body)-8] ^= 0xff
	require.NoError(t, os.WriteFile(path, body, 0o644))
}

func writeBinary(t *testing.T, path string, body []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, body, 0o644))
}
