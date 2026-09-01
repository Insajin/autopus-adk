package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A typo used to run the project's real test suite and report a passing check
// for a lane that does not exist.
func TestBuildPlanRejectsUnknownLane(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRunJourney(t, dir, "unit", "fast", "go-test")

	for _, lane := range []string{"totally-bogus-lane", "zzz", "Fast-ish"} {
		lane := lane
		t.Run(lane, func(t *testing.T) {
			t.Parallel()
			_, err := BuildPlan(Options{ProjectDir: dir, Lane: lane, Output: filepath.Join(dir, "runs")})

			require.Error(t, err)
			var laneErr *LaneError
			require.True(t, errors.As(err, &laneErr))
			assert.Equal(t, LaneErrorCode, laneErr.Code)
			assert.Equal(t, lane, laneErr.Lane)
			assert.Contains(t, laneErr.Known, "fast")
			assert.Contains(t, err.Error(), LaneErrorCode)
			assert.Contains(t, err.Error(), lane)
		})
	}
}

func TestBuildPlanAcceptsDeclaredAndCanonicalLanes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRunJourney(t, dir, "unit", "fast", "go-test")
	writeRunJourney(t, dir, "compliance", "acme-compliance", "go-test")

	for _, lane := range []string{"fast", "acme-compliance", "browser-staging", "canary-explicit", "mobile-scripted", "design-visual"} {
		lane := lane
		t.Run(lane, func(t *testing.T) {
			t.Parallel()
			plan, err := BuildPlan(Options{ProjectDir: dir, Lane: lane, Output: filepath.Join(dir, "runs")})

			require.NoError(t, err)
			assert.Equal(t, lane, plan.SelectedLane)
		})
	}
}

// A project-declared lane selects its own pack, not an auto-detected suite.
func TestBuildPlanSelectsProjectDeclaredCustomLane(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRunJourney(t, dir, "compliance", "acme-compliance", "go-test")

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "acme-compliance", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, []string{"compliance"}, plan.SelectedJourneys)
}

// The legitimate fallback survives: `fast` with no Journey Pack still runs what
// the harness detected.
func TestBuildPlanKeepsDetectedFallbackForFastWithoutPacks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n\ngo 1.24\n"), 0o600))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "fast", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Contains(t, plan.SelectedJourneys, "detected-go-test")
}

// An empty lane keeps normalizing to fast rather than erroring.
func TestBuildPlanNormalizesEmptyLane(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRunJourney(t, dir, "unit", "fast", "go-test")

	plan, err := BuildPlan(Options{ProjectDir: dir, Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, "fast", plan.SelectedLane)
	assert.Equal(t, []string{"unit"}, plan.SelectedJourneys)
}

// evidence-dashboard is a catalog lane with no adapter behind it. It must report
// the gap instead of borrowing a detected suite and calling it a pass.
func TestBuildPlanRefusesToSatisfyLaneWithoutExecutor(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n\ngo 1.24\n"), 0o600))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "evidence-dashboard", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Empty(t, plan.SelectedJourneys)
	assert.Empty(t, plan.SelectedAdapters)
	require.NotEmpty(t, plan.SetupGaps)
	assert.Contains(t, plan.SetupGaps[len(plan.SetupGaps)-1].Reason, "no adapter implements the evidence-dashboard lane")
}

// A project that supplies its own pack for the lane owns it and runs normally.
func TestBuildPlanRunsLaneWithoutExecutorWhenProjectDeclaresPack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRunJourney(t, dir, "dashboard", "evidence-dashboard", "go-test")

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "evidence-dashboard", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, []string{"dashboard"}, plan.SelectedJourneys)
}

func writeRunJourney(t *testing.T, dir, id, lane, adapterID string) {
	t.Helper()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := "id: " + id + "\n" +
		"title: " + id + "\n" +
		"surface: cli\n" +
		"lanes: [" + lane + "]\n" +
		"adapter:\n  id: " + adapterID + "\n" +
		"command:\n  run: go test ./...\n  cwd: .\n  timeout: 60s\n" +
		"checks:\n  - id: " + id + "\n    type: unit_test\n"
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, id+".yaml"), []byte(body), 0o600))
}
