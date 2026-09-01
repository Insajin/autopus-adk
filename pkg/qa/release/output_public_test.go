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

// The gate used to report warn + exit 0 over an index whose executed lanes had
// run_index_path:"" and manifest_paths:[""], because those refs were absolute
// and private-path redaction erased them. The evidence chain must survive an
// absolute --project-dir.
func TestExecuteKeepsEvidenceChainForAbsoluteProjectDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.True(t, filepath.IsAbs(dir))
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})

	payload, err := Execute(Options{
		ProjectDir: dir,
		Profile:    "prelaunch",
		Runner: LaneRunnerFunc(func(opts Options, lane string) (LaneRunResult, error) {
			// Mirror what the real runner returns now: project-relative refs.
			return LaneRunResult{
				Status:        LaneStatusPassed,
				RunIndexPath:  ".autopus/qa/runs/" + lane + "/run-index.json",
				ManifestPaths: []string{".autopus/qa/runs/" + lane + "/unit/manifest.json"},
			}, nil
		}),
	})
	require.NoError(t, err)

	fast := findLaneRow(t, payload.LaneRows, "fast")
	assert.Equal(t, ".autopus/qa/runs/fast/run-index.json", fast.RunIndexPath)
	require.Len(t, fast.ManifestPaths, 1)
	assert.NotEmpty(t, fast.ManifestPaths[0])

	for _, path := range []string{
		payload.OutputPaths.ReleaseIndexPath,
		payload.OutputPaths.RunIndexRoot,
		payload.OutputPaths.EvidenceRoot,
		payload.OutputPaths.FeedbackRoot,
	} {
		assert.NotEmpty(t, path)
		assert.NotEqual(t, qarun.RedactedLocalPath, path)
		assert.False(t, filepath.IsAbs(path), path)
	}

	body, readErr := os.ReadFile(filepath.Join(dir, payload.ReleaseIndexPath))
	require.NoError(t, readErr)
	var persisted Index
	require.NoError(t, json.Unmarshal(body, &persisted))
	require.Equal(t, "fast", persisted.LaneRows[0].Lane)
	assert.Equal(t, fast.RunIndexPath, persisted.LaneRows[0].RunIndexPath)
	assert.NotContains(t, string(body), dir)
}

// Dry-run publishes the same relative roots as the executed gate.
func TestBuildPlanPublishesRelativeOutputPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})

	plan, err := BuildPlan(Options{ProjectDir: dir, Profile: "prelaunch", DryRun: true})
	require.NoError(t, err)

	for _, path := range []string{
		plan.OutputPaths.ReleaseIndexPreviewPath,
		plan.OutputPaths.RunIndexRoot,
		plan.OutputPaths.EvidenceRoot,
		plan.OutputPaths.FeedbackRoot,
	} {
		assert.NotEmpty(t, path)
		assert.NotEqual(t, qarun.RedactedLocalPath, path)
		assert.False(t, filepath.IsAbs(path), path)
	}
}

// The placeholder must be emitted, not expanded away. "$PROJECT_ROOT" was read
// as a capture-group reference and every private path became "".
func TestRedactReleaseStringReplacesPrivatePathWithVisiblePlaceholder(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"/Users/alice/private/run-index.json",
		"/home/alice/private/manifest.json",
	} {
		got, changed := redactReleaseString(value)
		assert.True(t, changed, value)
		assert.NotEmpty(t, got, value)
		assert.Equal(t, qarun.RedactedLocalPath, got)
		assert.NotContains(t, got, "$")
	}
}
