package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				Status:       LaneStatusFailed,
				RunIndexPath: ".autopus/qa/runs/fast/run-index.json",
				FeedbackRefs: []string{".autopus/qa/feedback/fast-codex/bundle.json"},
			}, nil
		}),
	})
	require.ErrorIs(t, err, ErrReleaseBlocked)
	require.FileExists(t, payload.ReleaseIndexPath)

	body, readErr := os.ReadFile(payload.ReleaseIndexPath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(body), "command_preview_raw")

	assert.Equal(t, IndexSchemaVersion, payload.SchemaVersion)
	assert.Equal(t, GateStatusBlocked, payload.Status)
	assert.Len(t, payload.LaneRows, 7)
	fast := findLaneRow(t, payload.LaneRows, "fast")
	assert.Equal(t, LaneStatusFailed, fast.Status)
	assert.Equal(t, ".autopus/qa/runs/fast/run-index.json", fast.RunIndexPath)
	assert.Contains(t, fast.FeedbackRefs, ".autopus/qa/feedback/fast-codex/bundle.json")
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
	body, readErr := os.ReadFile(payload.ReleaseIndexPath)
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
