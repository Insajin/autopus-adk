package report

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
)

func TestBuild_ProjectsCaptureIndex(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	report, err := Build(fx.options())
	require.NoError(t, err)

	require.Len(t, report.Journeys, 1)
	journey := report.Journeys[0]
	require.NotNil(t, journey.Capture, journey.CaptureError)
	assert.Empty(t, journey.CaptureError)

	view := journey.Capture
	assert.Equal(t, capture.ModeAlways, view.Mode)
	assert.Equal(t, []string{"screenshot", "console", "network", "trace"}, view.Streams)
	assert.Equal(t, CaptureTotals{Steps: 2, Actions: 3, ConsoleErrors: 2, NetworkFailures: 1, Screenshots: 2, MediaBytes: fx.index.Totals.MediaBytes}, view.Totals)

	require.Len(t, view.Steps, 2)
	first, second := view.Steps[0], view.Steps[1]
	assert.Equal(t, "open-checkout", first.StepID)
	assert.Equal(t, 1, first.Order)
	assert.Equal(t, int64(3000), first.DurationMS)
	require.Len(t, first.Actions, 1)
	assert.Equal(t, "page.goto", first.Actions[0].API)
	require.NotNil(t, first.Console)
	assert.Equal(t, 1, first.Console.Warnings)
	assert.Equal(t, "hydration took 1200ms", first.Console.Messages[0].Text)
	require.NotNil(t, first.Network)
	assert.Equal(t, "/checkout", first.Network.Entries[0].URLRef)

	assert.Equal(t, capture.StatusFailed, second.Status)
	assert.Equal(t, "expired login stayed on checkout instead of redirecting", second.FailureSummary)
	assert.Equal(t, 2, second.Console.Errors)
	assert.Equal(t, 1, second.Network.Failures)
	assert.Equal(t, 500, second.Network.Entries[0].Status)

	// The filmstrip is positioned inside the journey's own 12s span.
	assert.InDelta(t, 0.0, first.Bar.OffsetPercent, 0.001)
	assert.InDelta(t, 25.0, first.Bar.WidthPercent, 0.001)
	assert.InDelta(t, 25.0, second.Bar.OffsetPercent, 0.001)
	assert.InDelta(t, 75.0, second.Bar.WidthPercent, 0.001)

	require.Len(t, view.Media, 1)
	assert.Equal(t, capture.StreamTrace, view.Media[0].Kind)
	assert.Equal(t, capture.RetentionLocalOnly, view.Media[0].Retention)
	assert.False(t, view.Media[0].Embedded)

	require.NotNil(t, view.Replay)
	assert.Equal(t, capture.ReplayKindPlaywrightGrep, view.Replay.Kind)
	assert.Equal(t, `npm exec playwright test --grep "checkout expired login"`, view.Replay.Command)
	assert.Equal(t, []string{"tests/e2e/checkout.spec.ts"}, view.Replay.SpecRefs)
	assert.Equal(t, 2, view.Replay.StepCount)

	// The raw index stays listed as evidence, but its JSON dump is superseded.
	artifact := artifactOfKind(t, journey, capture.ArtifactKind)
	assert.Empty(t, artifact.Preview)
	assert.Equal(t, "projected_as_capture", artifact.Withheld)
	assert.Positive(t, artifact.Bytes)

	// Suppressing that preview must not cost the other artifacts their budget.
	assert.Equal(t, "1 failing spec\n", artifactOfKind(t, journey, "stdout").Preview)

	assert.Equal(t, 2, report.Summary.CaptureStepCount)
	assert.Equal(t, 2, report.Summary.ConsoleErrors)
	assert.Equal(t, 1, report.Summary.NetworkFailures)
	assert.Equal(t, 2, report.Summary.ScreenshotCount)
	assert.Equal(t, RetentionShareable, report.Retention)
}

func TestBuild_KeepsRenderingWhenCaptureIndexIsInvalid(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	fx.rewriteIndex(t, func(index *capture.Index) { index.Totals.Steps = 99 })

	report, err := Build(fx.options())
	require.NoError(t, err)

	require.Len(t, report.Journeys, 1)
	journey := report.Journeys[0]
	assert.Nil(t, journey.Capture)
	assert.Contains(t, journey.CaptureError, "invalid_capture_index:")
	assert.Contains(t, journey.CaptureError, "totals disagree with steps")

	// The report still stands, and the raw index is inlined for diagnosis.
	assert.Equal(t, IngestionComplete, report.Ingestion.Status)
	assert.Equal(t, 0, report.Summary.CaptureStepCount)
	assert.Contains(t, artifactOfKind(t, journey, capture.ArtifactKind).Preview, capture.IndexSchemaVersion)
}

func TestBuild_CaptureIndexRefusedWhenMissing(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	require.NoError(t, os.Remove(fx.indexPath))

	report, err := Build(fx.options())
	require.NoError(t, err)

	journey := report.Journeys[0]
	assert.Nil(t, journey.Capture)
	assert.Equal(t, "missing_capture_index", journey.CaptureError)
}

func TestBuild_DoesNotEmbedMediaByDefault(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	report, err := Build(fx.options())
	require.NoError(t, err)

	shot := report.Journeys[0].Capture.Steps[0].Screenshot
	require.NotNil(t, shot)
	assert.False(t, shot.Embedded)
	assert.Empty(t, shot.DataURI)
	assert.Empty(t, shot.MediaError)
	// Metadata is what a shareable report carries: identity, not pixels.
	assert.Equal(t, fx.index.Steps[0].Screenshot.Digest, shot.Digest)
	assert.Equal(t, 8, shot.Width)
	assert.Equal(t, 5, shot.Height)
	assert.Equal(t, RetentionShareable, report.Retention)
}

func TestBuild_EmbedsMediaOnRequestAndDowngradesRetention(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	report, err := Build(fx.embedOptions())
	require.NoError(t, err)

	for _, step := range report.Journeys[0].Capture.Steps {
		require.NotNil(t, step.Screenshot)
		assert.True(t, step.Screenshot.Embedded, step.StepID)
		assert.Empty(t, step.Screenshot.MediaError)
		assert.True(t, strings.HasPrefix(step.Screenshot.DataURI, "data:image/png;base64,"), step.StepID)
	}
	// Journey-level media is never inlined, whatever the caller asked for.
	assert.False(t, report.Journeys[0].Capture.Media[0].Embedded)
	assert.Equal(t, RetentionLocalOnly, report.Retention)
}

func TestBuild_RefusesToEmbedWhenDigestDisagrees(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	corruptShot(t, fx.shotPaths[0])

	report, err := Build(fx.embedOptions())
	require.NoError(t, err)

	steps := report.Journeys[0].Capture.Steps
	assert.False(t, steps[0].Screenshot.Embedded)
	assert.Empty(t, steps[0].Screenshot.DataURI)
	assert.Equal(t, "digest_mismatch", steps[0].Screenshot.MediaError)
	// The untouched step still embeds, so the refusal is per file.
	assert.True(t, steps[1].Screenshot.Embedded)
	assert.Equal(t, RetentionLocalOnly, report.Retention)
}

func TestBuild_RejectsCaptureIndexWithEscapingMediaRef(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	fx.rewriteIndex(t, func(index *capture.Index) {
		index.Steps[0].Screenshot.Ref = "../../etc/passwd"
	})

	report, err := Build(fx.embedOptions())
	require.NoError(t, err)

	// The capture contract refuses the traversal before the report ever reads it.
	journey := report.Journeys[0]
	assert.Nil(t, journey.Capture)
	assert.Contains(t, journey.CaptureError, "must not escape the capture directory")
	assert.Equal(t, RetentionShareable, report.Retention)

	var out strings.Builder
	require.NoError(t, Render(&out, report))
	assert.NotContains(t, out.String(), "data:image")
}

// TestMediaView_ConfinesRefsIndependentlyOfValidation covers the second gate: a
// ref that reached mediaView without passing capture.Validate must still be
// refused, because confinement is the report's own invariant.
func TestMediaView_ConfinesRefsIndependentlyOfValidation(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	declared := *fx.index.Steps[0].Screenshot
	for _, ref := range []string{"../../etc/passwd", "media/../../../etc/passwd", "/etc/passwd"} {
		escaping := declared
		escaping.Ref = ref
		view := mediaView(capture.StreamScreenshot, fx.captureDir, escaping, newMediaBudget(true))
		assert.False(t, view.Embedded, ref)
		assert.Empty(t, view.DataURI, ref)
		assert.Contains(t, []string{"unsafe_media_ref", "unsupported_media_format"}, view.MediaError, ref)
	}

	// The declared ref itself embeds, so the refusals above are about the path.
	ok := mediaView(capture.StreamScreenshot, fx.captureDir, declared, newMediaBudget(true))
	assert.True(t, ok.Embedded)
}

func TestMediaView_RefusesUnsupportedFormatAndOversizeBudget(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	trace := fx.index.Media[0].MediaRef
	assert.Equal(t, "unsupported_media_format", mediaView(capture.StreamTrace, fx.captureDir, trace, newMediaBudget(true)).MediaError)

	exhausted := newMediaBudget(true)
	exhausted.bytesLeft = 1
	shot := mediaView(capture.StreamScreenshot, fx.captureDir, *fx.index.Steps[0].Screenshot, exhausted)
	assert.Equal(t, "embed_budget_exhausted", shot.MediaError)
	assert.False(t, shot.Embedded)
}

// TestBuild_ResolvesMediaFromLocalCaptureDirNotArtifactDir pins the resolution
// root. Only the sanitized index is published under artifacts/; the raw bytes
// live in the run's local capture directory. A projection that resolved refs
// beside the artifact would embed the decoy bytes, or nothing at all.
func TestBuild_ResolvesMediaFromLocalCaptureDirNotArtifactDir(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	require.FileExists(t, fx.decoyPaths[0])
	require.Contains(t, fx.captureDir, filepath.Join(capture.RawDirName, "gui-capture", capture.DirName))

	report, err := Build(fx.embedOptions())
	require.NoError(t, err)

	raw, err := os.ReadFile(fx.shotPaths[0])
	require.NoError(t, err)
	shot := report.Journeys[0].Capture.Steps[0].Screenshot
	require.NotNil(t, shot)
	require.True(t, shot.Embedded, shot.MediaError)
	assert.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw), shot.DataURI)
	assert.Equal(t, RetentionLocalOnly, report.Retention)
}

// TestBuild_ReportsPrunedLocalCaptureMedia covers the run whose raw tree was
// cleaned: the published index still projects, the pixels are simply gone.
func TestBuild_ReportsPrunedLocalCaptureMedia(t *testing.T) {
	t.Parallel()

	fx := newCaptureFixture(t)
	require.NoError(t, os.RemoveAll(fx.captureDir))

	report, err := Build(fx.embedOptions())
	require.NoError(t, err)

	journey := report.Journeys[0]
	require.NotNil(t, journey.Capture, journey.CaptureError)
	assert.Equal(t, "missing_media_file", journey.Capture.Steps[0].Screenshot.MediaError)
	assert.False(t, journey.Capture.Steps[0].Screenshot.Embedded)
	assert.Equal(t, RetentionShareable, report.Retention)
}

func artifactOfKind(t *testing.T, journey JourneyView, kind string) ArtifactView {
	t.Helper()
	for _, artifact := range journey.Artifacts {
		if artifact.Kind == kind {
			return artifact
		}
	}
	t.Fatalf("journey %s declares no %s artifact", journey.JourneyID, kind)
	return ArtifactView{}
}
