package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestExecuteWritesReleaseIndexWhenFirstLaneBlocks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})
	output := filepath.Join(dir, ".autopus", "qa", "releases")

	payload, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Output:     output,
		Command:    "auto qa release --profile prelaunch --format json",
		Runner: LaneRunnerFunc(func(_ Options, lane string) (LaneRunResult, error) {
			require.Equal(t, "fast", lane)
			return LaneRunResult{
				Status:          LaneStatusFailed,
				RunIndexPath:    ".autopus/qa/runs/fast/run-index.json",
				ManifestPaths:   []string{".autopus/qa/runs/fast/unit/manifest.json"},
				FeedbackRefs:    []string{".autopus/qa/feedback/fast-codex/bundle.json"},
				FailedJourneyID: "unit",
				FailureSummary:  "expected exit_code=0",
			}, nil
		}),
	})
	require.ErrorIs(t, err, ErrReleaseBlocked)
	// Published refs are project-root-relative; a consumer resolves them against
	// the workspace root instead of receiving an absolute local path.
	assert.False(t, filepath.IsAbs(payload.ReleaseIndexPath))
	releaseIndexPath := filepath.Join(dir, payload.ReleaseIndexPath)
	require.FileExists(t, releaseIndexPath)

	body, readErr := os.ReadFile(releaseIndexPath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(body), "command_preview_raw")

	assert.Equal(t, IndexSchemaVersion, payload.SchemaVersion)
	assert.Equal(t, GateStatusBlocked, payload.Status)
	assert.Equal(t, filepath.Base(dir), payload.Workspace.WorkspaceID)
	assert.Equal(t, filepath.Base(dir), payload.Workspace.RepoID)
	assert.Equal(t, ".", payload.Workspace.RepoRoot)
	assert.Contains(t, payload.SourceRefs, "qamesh://source/"+filepath.Base(dir)+"/specs/SPEC-QAMESH-002")
	assert.Len(t, payload.LaneRows, 7)
	fast := findLaneRow(t, payload.LaneRows, "fast")
	assert.Equal(t, LaneStatusFailed, fast.Status)
	assert.Equal(t, ".autopus/qa/runs/fast/run-index.json", fast.RunIndexPath)
	assert.Equal(t, "unit", fast.FailedJourneyID)
	assert.Equal(t, "expected exit_code=0", fast.FailureSummary)
	assert.Contains(t, fast.ManifestPaths, ".autopus/qa/runs/fast/unit/manifest.json")
	assert.Contains(t, fast.FeedbackRefs, ".autopus/qa/feedback/fast-codex/bundle.json")
	if assert.Len(t, fast.Blockers, 1) {
		assert.Equal(t, "journey_failed:unit", fast.Blockers[0].Reason)
	}
	assert.True(t, fast.DeterministicAuthority)
	desktop := findLaneRow(t, payload.LaneRows, "desktop-native")
	assert.Equal(t, LaneStatusSkipped, desktop.Status)
	assert.Equal(t, "not_started_after_block", desktop.SkippedReason)
}

func TestExecuteAggregatesOptionalSetupGapAsWarn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, lane := range []string{"fast", "browser-staging", "desktop-native"} {
		writeReleaseJourney(t, dir, lane, lane, "go-test", []string{"go", "test", "./..."})
	}
	writeReleaseJourney(t, dir, "local-gui", "gui-explore", "gui-explore", []string{"npx", "playwright", "test"})

	payload, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Output:     filepath.Join(dir, ".autopus", "qa", "releases"),
		Runner: LaneRunnerFunc(func(_ Options, lane string) (LaneRunResult, error) {
			return LaneRunResult{Status: LaneStatusPassed, RunIndexPath: ".autopus/qa/runs/" + lane + "/run-index.json"}, nil
		}),
	})
	require.NoError(t, err)

	assert.Equal(t, GateStatusWarn, payload.Status)
	canary := findLaneRow(t, payload.LaneRows, "canary-explicit")
	assert.Equal(t, LaneStatusSetupGap, canary.Status)
	assert.Equal(t, SetupGapCanaryTemplate, canary.SetupGapClass)
	assert.Equal(t, LaneVerdictWarn, canary.LaneVerdict)
	assert.NotNil(t, canary.Blockers)
}

// A deferred lane must stay visible and non-blocking. Suppressing its setup gap
// row made mobile-readiness report verdict=pass with setup_gap_class=none while
// `auto qa run --lane mobile-readiness` fail-closed on the same project.
func TestExecuteKeepsDeferredLaneGapsVisibleWithoutBlocking(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, lane := range []string{"fast", "browser-staging", "desktop-native", "gui-explore", "canary-explicit"} {
		adapter := "go-test"
		argv := []string{"go", "test", "./..."}
		if lane == "gui-explore" {
			adapter = "gui-explore"
			argv = []string{"npx", "playwright", "test"}
		}
		if lane == "canary-explicit" {
			adapter = "canary-template"
			argv = []string{"auto", "canary"}
		}
		writeReleaseJourney(t, dir, lane, lane, adapter, argv)
	}

	payload, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Output:     filepath.Join(dir, ".autopus", "qa", "releases"),
		Runner: LaneRunnerFunc(func(_ Options, lane string) (LaneRunResult, error) {
			return LaneRunResult{Status: LaneStatusPassed, RunIndexPath: ".autopus/qa/runs/" + lane + "/run-index.json"}, nil
		}),
	})
	require.NoError(t, err)

	assert.Equal(t, GateStatusWarn, payload.Status)
	assert.Empty(t, payload.Blockers)
	assertReleaseGap(t, payload.SetupGaps, "mobile-readiness", SetupGapEnvMissing, false, SeverityMedium)
	assertReleaseGap(t, payload.SetupGaps, "evidence-dashboard", SetupGapLaneNotImplemented, false, SeverityMedium)
	mobile := findLaneRow(t, payload.LaneRows, "mobile-readiness")
	assert.Equal(t, LaneStatusDeferred, mobile.Status)
	assert.Equal(t, SetupGapEnvMissing, mobile.SetupGapClass)
	assert.Equal(t, LaneVerdictWarn, mobile.LaneVerdict)
	assert.Empty(t, mobile.Blockers)
	dashboard := findLaneRow(t, payload.LaneRows, "evidence-dashboard")
	assert.Equal(t, SetupGapLaneNotImplemented, dashboard.SetupGapClass)
	assert.NotEqual(t, LaneVerdictPass, dashboard.LaneVerdict)
	assert.Empty(t, dashboard.Blockers)
}

func TestExecuteKeepsAIAnalysisUntrustedForVerdict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})
	writeReleaseJourney(t, dir, "browser", "browser-staging", "go-test", []string{"go", "test", "./..."})

	payload, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Output:     filepath.Join(dir, ".autopus", "qa", "releases"),
		Runner: LaneRunnerFunc(func(_ Options, lane string) (LaneRunResult, error) {
			if lane == "browser-staging" {
				return LaneRunResult{
					Status:         LaneStatusFailed,
					RunIndexPath:   ".autopus/qa/runs/browser/run-index.json",
					AIAnalysisRefs: []AIAnalysisRef{{Ref: "ai://browser-ok", TrustedForVerdict: true}},
				}, nil
			}
			return LaneRunResult{Status: LaneStatusPassed, RunIndexPath: ".autopus/qa/runs/" + lane + "/run-index.json"}, nil
		}),
	})
	require.ErrorIs(t, err, ErrReleaseBlocked)

	browser := findLaneRow(t, payload.LaneRows, "browser-staging")
	assert.Equal(t, LaneStatusFailed, browser.Status)
	assert.True(t, browser.DeterministicAuthority)
	assert.Equal(t, GateStatusBlocked, payload.Status)
	if assert.Len(t, payload.AIAnalysisRefs, 1) {
		assert.False(t, payload.AIAnalysisRefs[0].TrustedForVerdict)
	}
}

func TestExecuteRedactsReturnedRefsInPayloadAndReleaseIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})

	payload, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Output:     filepath.Join(dir, ".autopus", "qa", "releases"),
		Runner: LaneRunnerFunc(func(_ Options, lane string) (LaneRunResult, error) {
			require.Equal(t, "fast", lane)
			return LaneRunResult{
				Status:        LaneStatusFailed,
				RunIndexPath:  "/Users/alice/private/API_TOKEN=s3cr3t/run-index.json?token=tok_value",
				ManifestPaths: []string{"/home/alice/private/password=hunter2/manifest.json"},
				FeedbackRefs:  []string{"https://user:pass@example.test/out?api_key=secret_value"},
				AIAnalysisRefs: []AIAnalysisRef{{
					Ref:               "/Users/alice/ai?secret=ai_secret_value",
					TrustedForVerdict: true,
				}},
			}, nil
		}),
	})
	require.ErrorIs(t, err, ErrReleaseBlocked)
	body, readErr := os.ReadFile(filepath.Join(dir, payload.ReleaseIndexPath))
	require.NoError(t, readErr)
	for _, text := range []string{string(body), payload.LaneRows[0].RunIndexPath, payload.FeedbackRefs[0], payload.AIAnalysisRefs[0].Ref} {
		assert.NotContains(t, text, "/Users/alice")
		assert.NotContains(t, text, "/home/alice")
		assert.NotContains(t, text, "s3cr3t")
		assert.NotContains(t, text, "hunter2")
		assert.NotContains(t, text, "user:pass@")
		assert.NotContains(t, text, "secret_value")
		assert.NotContains(t, text, "ai_secret_value")
	}
	assert.NotContains(t, string(body), "API_TOKEN=s3cr3t")
	// A NotContains assertion also passes on "". Redaction must replace the
	// private path with a visible placeholder, not erase the ref of a lane that
	// actually ran. Deferred lanes legitimately carry "" and are not asserted on.
	assert.Equal(t, qarun.RedactedLocalPath, payload.LaneRows[0].RunIndexPath)
	require.Len(t, payload.LaneRows[0].ManifestPaths, 1)
	assert.Equal(t, qarun.RedactedLocalPath, payload.LaneRows[0].ManifestPaths[0])
	var persisted Index
	require.NoError(t, json.Unmarshal(body, &persisted))
	require.Equal(t, "fast", persisted.LaneRows[0].Lane)
	assert.Equal(t, qarun.RedactedLocalPath, persisted.LaneRows[0].RunIndexPath)
	require.Len(t, persisted.LaneRows[0].ManifestPaths, 1)
	assert.Equal(t, qarun.RedactedLocalPath, persisted.LaneRows[0].ManifestPaths[0])
	assert.Equal(t, RedactionRedacted, payload.RedactionStatus)
}

func TestExecuteDefaultRunnerRedactsRetainedRunArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{
		"go", "test", "./...", "--password", "hunter2",
		"--api-key=abc12345",
		"--report", "https://user:pass@example.test/out?token=tok12345",
		"--config", "/Users/alice/private.env",
	})

	_, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Output:     filepath.Join(dir, ".autopus", "qa", "releases"),
	})
	require.ErrorIs(t, err, ErrReleaseBlocked)

	paths, globErr := filepath.Glob(filepath.Join(dir, ".autopus", "qa", "runs", "*", "unit", "manifest.json"))
	require.NoError(t, globErr)
	require.Len(t, paths, 1)
	assert.NoDirExists(t, filepath.Join(filepath.Dir(filepath.Dir(paths[0])), "_raw"))
	body, readErr := os.ReadFile(paths[0])
	require.NoError(t, readErr)
	text := string(body)
	for _, secret := range []string{"hunter2", "abc12345", "user:pass@", "tok12345", "/Users/alice"} {
		assert.NotContains(t, text, secret)
	}
	assert.Contains(t, text, "[REDACTED_SECRET]")
}
