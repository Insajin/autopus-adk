package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_IsSelfContained(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	writeReleaseIndex(t, fx.root)
	report, err := Build(fx.options())
	require.NoError(t, err)

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	html := out.String()

	// A shareable report must not reach the network or execute anything.
	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "src=")
	assert.NotContains(t, html, "http://")
	assert.NotContains(t, html, "https://")
	assert.Contains(t, html, `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">`)

	// The stylesheet is inlined rather than linked.
	assert.Contains(t, html, "<style>")
	assert.Contains(t, html, "--pass:")
	assert.NotContains(t, html, "<link")

	// Nothing was dropped by the CSS value filter in the timeline bars.
	assert.NotContains(t, html, "ZgotmplZ")
	assert.Contains(t, html, "width:75.000%")
}

func TestRender_ShowsEveryEvidenceSection(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	writeReleaseIndex(t, fx.root)
	report, err := Build(fx.options())
	require.NoError(t, err)

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	html := out.String()

	assert.Contains(t, html, "cli-fast")
	assert.Contains(t, html, "gui-checkout")
	assert.Contains(t, html, "expired sessions are not redirected")
	assert.Contains(t, html, "ok 12 tests")
	assert.Contains(t, html, "[REDACTED_SECRET]")
	assert.Contains(t, html, "withheld: local_only_evidence")
	assert.Contains(t, html, "Release gate")
	assert.Contains(t, html, "journey pack missing")
	assert.Contains(t, html, "Timeline")
	assert.Contains(t, html, `class="pill pill-lg v-failed"`)
	// The provider token must never survive into the rendered document.
	assert.NotContains(t, html, "sk-live-abcdefghijklmnopqrst")
}

func TestRender_EscapesEvidenceContent(t *testing.T) {
	t.Parallel()

	report := Report{
		SchemaVersion: SchemaVersion,
		Title:         `Report <script>alert(1)</script>`,
		Verdict:       VerdictFailed,
		Ingestion:     Ingestion{Status: IngestionComplete},
		Journeys: []JourneyView{{
			JourneyID:      `<img src=x onerror=alert(1)>`,
			Surface:        "frontend",
			Lane:           "gui-explore",
			Verdict:        VerdictFailed,
			RunnerName:     "gui-explore",
			RetentionClass: "local-redacted",
			FailureSummary: `<b>bold</b>`,
			Artifacts: []ArtifactView{{
				Kind:        "console_summary",
				Ref:         "artifacts/console.json",
				Publishable: true,
				Redaction:   "text_redacted_and_scanned",
				Preview:     "</pre><script>evil()</script>",
			}},
		}},
	}

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	html := out.String()

	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "<img src=x")
	assert.NotContains(t, html, "<b>bold</b>")
	assert.Contains(t, html, "&lt;script&gt;evil()&lt;/script&gt;")
	assert.Contains(t, html, "&lt;img src=x onerror=alert(1)&gt;")
}

func TestWriteFile_WritesAtomicallyBesideRunIndex(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	report, err := Build(fx.options())
	require.NoError(t, err)

	path := DefaultOutputPath(fx.runIndexPath)
	assert.Equal(t, filepath.Join(fx.runDir, DefaultReportFile), path)
	require.NoError(t, WriteFile(report, path))

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(body), "<!DOCTYPE html>"))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	// No temporary render artifact is left behind.
	entries, err := os.ReadDir(fx.runDir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, strings.HasPrefix(entry.Name(), ".qa-report-"), entry.Name())
	}
}

func TestRender_StatesBlockedIngestion(t *testing.T) {
	t.Parallel()

	report := Report{
		SchemaVersion: SchemaVersion,
		Title:         "blocked run",
		Verdict:       VerdictBlocked,
		Ingestion: Ingestion{
			Status:     IngestionBlocked,
			Rejections: []Rejection{{Ref: "qa/run-index.json", Reason: "redaction_not_passed:blocked"}},
		},
	}

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	html := out.String()

	assert.Contains(t, html, "banner banner-blocked")
	assert.Contains(t, html, "redaction_not_passed:blocked")
	assert.Contains(t, html, "No evidence manifest was ingested for this run.")
}

func TestRender_ShowsCaptureFilmstripWithoutInliningMedia(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	report, err := Build(fx.options())
	require.NoError(t, err)

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	html := out.String()

	assert.Contains(t, html, `<div class="capture">`)
	assert.Contains(t, html, "open-checkout")
	assert.Contains(t, html, "submit-expired")
	assert.Contains(t, html, "expired login stayed on checkout instead of redirecting")
	assert.Contains(t, html, `class="sev-error"`)
	assert.Contains(t, html, "POST /orders failed with 500")
	assert.Contains(t, html, "/orders")
	assert.Contains(t, html, `npm exec playwright test --grep &#34;checkout expired login&#34;`)
	assert.Contains(t, html, "tests/e2e/checkout.spec.ts")
	assert.Contains(t, html, "retention shareable")

	// A shareable report names the pixels it refuses to carry.
	assert.Contains(t, html, "no image inlined")
	assert.Contains(t, html, fx.index.Steps[0].Screenshot.Digest[:19])
	assert.Contains(t, html, "8×5")
	assert.NotContains(t, html, "data:image")
	assert.NotContains(t, html, "<img")

	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "<link")
	assert.NotContains(t, html, "http://")
	assert.NotContains(t, html, "https://")
	assert.NotContains(t, html, "ZgotmplZ")
}

func TestRender_EmbedsMediaBehindLocalOnlyBanner(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	report, err := Build(fx.embedOptions())
	require.NoError(t, err)

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	html := out.String()

	assert.Contains(t, html, "data:image/png;base64,")
	assert.Contains(t, html, `loading="lazy"`)
	assert.Contains(t, html, `alt="Open checkout"`)
	assert.Contains(t, html, "banner banner-local")
	assert.Contains(t, html, "do not publish, attach, or share this file")
	assert.Contains(t, html, "retention local-only")
	// The data URL must survive html/template's URL filter intact.
	assert.NotContains(t, html, "ZgotmplZ")

	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "<link")
	assert.NotContains(t, html, "http://")
	assert.NotContains(t, html, "https://")
}

func TestHumanDuration(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "—", humanDuration(0))
	assert.Equal(t, "250ms", humanDuration(250))
	assert.Equal(t, "1.5s", humanDuration(1500))
	assert.Equal(t, "2m 05s", humanDuration(125_000))
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "0 B", humanBytes(0))
	assert.Equal(t, "512 B", humanBytes(512))
	assert.Equal(t, "1.5 KB", humanBytes(1536))
	assert.Equal(t, "2.0 MB", humanBytes(2*1024*1024))
}

func TestSanitizeText_StripsControlAndAnsi(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pass", sanitizeText("\x1b[32mpass\x1b[0m"))
	assert.Equal(t, "a\nb", sanitizeText("a\r\nb"))
	assert.Equal(t, "ab", sanitizeText("a\x00b"))
	assert.Equal(t, "a\tb", sanitizeText("a\tb"))
}
