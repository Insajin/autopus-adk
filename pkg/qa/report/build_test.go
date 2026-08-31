package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestBuild_ProjectsRunEvidence(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	report, err := Build(fx.options())
	require.NoError(t, err)

	assert.Equal(t, SchemaVersion, report.SchemaVersion)
	assert.Equal(t, VerdictFailed, report.Verdict)
	assert.Equal(t, IngestionComplete, report.Ingestion.Status)
	assert.Equal(t, 2, report.Ingestion.ManifestCount)
	require.NotNil(t, report.Run)
	assert.Equal(t, fixtureRunID, report.Run.RunID)
	assert.Equal(t, "passed", report.Run.RedactionStatus)

	assert.Equal(t, 2, report.Summary.JourneyCount)
	assert.Equal(t, 1, report.Summary.JourneysPassed)
	assert.Equal(t, 1, report.Summary.JourneysFailed)
	assert.Equal(t, 2, report.Summary.CheckCount)
	assert.Equal(t, 1, report.Summary.ChecksFailed)
	assert.Equal(t, 4, report.Summary.ArtifactCount)
	assert.Equal(t, 1, report.Summary.SetupGapCount)
	assert.Equal(t, int64(20000), report.Summary.DurationMS)

	// Journeys are ordered by start time so the timeline reads top to bottom.
	require.Len(t, report.Journeys, 2)
	assert.Equal(t, "cli-fast", report.Journeys[0].JourneyID)
	assert.Equal(t, "gui-checkout", report.Journeys[1].JourneyID)
	assert.Equal(t, VerdictPassed, report.Journeys[0].Verdict)
	assert.Equal(t, "expired sessions are not redirected", report.Journeys[1].FailureSummary)
	assert.Equal(t, "step-1", report.Journeys[1].StepID)
	assert.Equal(t, manifestRef("gui-checkout"), report.Journeys[1].ManifestRef)

	assert.Equal(t, int64(20000), report.Timeline.SpanMS)
	assert.InDelta(t, 0.0, report.Journeys[0].Bar.OffsetPercent, 0.001)
	assert.InDelta(t, 25.0, report.Journeys[1].Bar.OffsetPercent, 0.001)
	assert.InDelta(t, 75.0, report.Journeys[1].Bar.WidthPercent, 0.001)
}

func TestBuild_InlinesPublishableTextAndWithholdsTheRest(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	report, err := Build(fx.options())
	require.NoError(t, err)

	byKind := map[string]ArtifactView{}
	for _, journey := range report.Journeys {
		for _, artifact := range journey.Artifacts {
			byKind[artifact.Kind] = artifact
		}
	}

	stdout := byKind["stdout"]
	assert.Equal(t, "ok 12 tests\n", stdout.Preview)
	assert.False(t, stdout.Truncated)
	assert.Empty(t, stdout.Withheld)

	// A provider token inside a publishable artifact must be redacted, never rendered.
	console := byKind["console_summary"]
	assert.NotContains(t, console.Preview, "sk-live-abcdefghijklmnopqrst")
	assert.Contains(t, console.Preview, evidence.RedactedSecret)

	// Local-only quarantine refs stay metadata-only even though the file exists.
	shot := byKind["screenshot_quarantine_ref"]
	assert.Empty(t, shot.Preview)
	assert.Equal(t, "local_only_evidence", shot.Withheld)

	// Publishable but non-text media is never inlined.
	clip := byKind["video_quarantine_ref"]
	assert.Empty(t, clip.Preview)
	assert.Equal(t, "unsupported_format", clip.Withheld)
}

func TestBuild_MarksEmptyAndMissingArtifacts(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	stdout := filepath.Join(fx.runDir, "cli-fast", "artifacts", "stdout", "stdout.log")
	require.NoError(t, os.WriteFile(stdout, []byte("   \n"), 0o644))
	require.NoError(t, os.Remove(filepath.Join(fx.runDir, "gui-checkout", "artifacts", "console_summary", "console.json")))

	report, err := Build(fx.options())
	require.NoError(t, err)

	byKind := map[string]ArtifactView{}
	for _, journey := range report.Journeys {
		for _, artifact := range journey.Artifacts {
			byKind[artifact.Kind] = artifact
		}
	}
	assert.Equal(t, "empty_artifact", byKind["stdout"].Withheld)
	assert.Equal(t, "missing_file", byKind["console_summary"].Withheld)
	assert.Equal(t, 0, report.Summary.PreviewCount)
	assert.Equal(t, 4, report.Summary.WithheldCount)
}

func TestBuild_RejectsManifestOutsideProjectRoot(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	outside := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(outside, []byte("{}"), 0o644))
	patchRunIndex(t, fx.runIndexPath, func(index *qarun.Index) {
		index.ManifestPaths = append(index.ManifestPaths, outside)
	})

	report, err := Build(fx.options())
	require.NoError(t, err)
	assert.Equal(t, IngestionDegraded, report.Ingestion.Status)
	require.Len(t, report.Ingestion.Rejections, 1)
	assert.Equal(t, "unsafe_ref:absolute_outside_project", report.Ingestion.Rejections[0].Reason)
	assert.Equal(t, 2, report.Ingestion.ManifestCount)
}

func TestBuild_BlocksWhenRunIndexRedactionDidNotPass(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	patchRunIndex(t, fx.runIndexPath, func(index *qarun.Index) {
		index.RedactionStatus = qarun.RedactionStatus{Status: "blocked"}
	})

	report, err := Build(fx.options())
	require.NoError(t, err)
	assert.Equal(t, IngestionBlocked, report.Ingestion.Status)
	assert.Equal(t, VerdictBlocked, report.Verdict)
	assert.Empty(t, report.Journeys)
	require.Len(t, report.Ingestion.Rejections, 1)
	assert.Equal(t, "redaction_not_passed:blocked", report.Ingestion.Rejections[0].Reason)
}

func TestBuild_BlocksOnUnsupportedRunIndexSchema(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	patchRunIndex(t, fx.runIndexPath, func(index *qarun.Index) {
		index.SchemaVersion = "qamesh.run_index.v99"
	})

	report, err := Build(fx.options())
	require.NoError(t, err)
	assert.Equal(t, IngestionBlocked, report.Ingestion.Status)
	assert.Nil(t, report.Run)
	require.Len(t, report.Ingestion.Rejections, 1)
	assert.Equal(t, "unsupported_schema_version:qamesh.run_index.v99", report.Ingestion.Rejections[0].Reason)
}

func TestBuild_RejectsInvalidManifest(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	path := filepath.Join(fx.runDir, "cli-fast", "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"schema_version":"qamesh.evidence.v2"}`), 0o644))

	report, err := Build(fx.options())
	require.NoError(t, err)
	assert.Equal(t, IngestionDegraded, report.Ingestion.Status)
	require.Len(t, report.Ingestion.Rejections, 1)
	assert.Contains(t, report.Ingestion.Rejections[0].Reason, "invalid_manifest:")
	assert.Equal(t, 1, report.Ingestion.ManifestCount)
}

func TestBuild_SurfacesAdapterResultWithoutManifest(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	patchRunIndex(t, fx.runIndexPath, func(index *qarun.Index) {
		index.ManifestPaths = index.ManifestPaths[:1]
		index.AdapterResults[1].QAMESHManifestPath = ""
	})

	report, err := Build(fx.options())
	require.NoError(t, err)
	require.Len(t, report.Journeys, 2)
	missing := report.Journeys[1]
	assert.Equal(t, "gui-checkout", missing.JourneyID)
	assert.Equal(t, VerdictBlocked, missing.Verdict)
	assert.Equal(t, "expired sessions are not redirected", missing.FailureSummary)
}

func TestBuild_IncludesReleaseGateMatrix(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	writeReleaseIndex(t, fx.root)

	report, err := Build(fx.options())
	require.NoError(t, err)
	require.NotNil(t, report.Release)
	assert.Equal(t, fixtureReleaseID, report.Release.ReleaseID)
	assert.Equal(t, "blocked", report.Release.Status)
	require.Len(t, report.Release.Lanes, 2)
	assert.Equal(t, "block", report.Release.Lanes[1].Verdict)
	require.Len(t, report.Release.Blockers, 1)
	assert.Equal(t, "gui-explore: must lane failed", report.Release.Blockers[0])
	assert.Equal(t, 2, report.Summary.LaneCount)
	// Release setup gaps join the run gaps so one list explains every blocked lane.
	assert.Equal(t, 2, report.Summary.SetupGapCount)
}

func TestBuild_FailsWithoutRunIndex(t *testing.T) {
	t.Parallel()

	_, err := Build(Options{ProjectDir: t.TempDir(), Now: fixedNow})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no QAMESH run index found")
}

func TestResolveLatestIndex_PrefersNewest(t *testing.T) {
	t.Parallel()

	fx := newFixture(t)
	older := filepath.Join(fx.root, ".autopus", "qa", "runs", "qa-older", RunIndexFile)
	writeFile(t, older, "{}")
	require.NoError(t, os.Chtimes(older, fixedNow(), fixedNow().Add(-time.Hour)))

	resolved, err := ResolveLatestIndex(fx.root, RunsRelDir, RunIndexFile)
	require.NoError(t, err)
	assert.Equal(t, fx.runIndexPath, resolved)
}

func patchRunIndex(t *testing.T, path string, mutate func(*qarun.Index)) {
	t.Helper()
	var index qarun.Index
	require.NoError(t, readIndexJSON(path, &index))
	mutate(&index)
	writeJSON(t, path, index)
}
