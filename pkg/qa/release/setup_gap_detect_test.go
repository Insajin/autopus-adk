package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qalane "github.com/insajin/autopus-adk/pkg/qa/lane"
)

// Every shipped profile must be able to report that mobile-readiness has unmet
// preconditions. Before the fix the gap row was suppressed in all three, so the
// only lane that fail-closes on a direct `auto qa run` scored a silent pass.
func TestReleasePlanReportsMobileReadinessGapInEveryProfile(t *testing.T) {
	t.Parallel()

	for _, profile := range []string{"prelaunch", "release-candidate", "postdeploy-smoke"} {
		profile := profile
		t.Run(profile, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeReleaseJourney(t, dir, "unit", "fast", "go-test", []string{"go", "test", "./..."})

			plan, err := BuildPlan(Options{ProjectDir: dir, Profile: profile, DryRun: true})
			require.NoError(t, err)

			gap := findSetupGapRow(t, plan.SetupGaps, "mobile-readiness")
			assert.Equal(t, SetupGapEnvMissing, gap.SetupGapClass)
			assert.Contains(t, gap.Reason, "missing_device_inventory")
			assert.Contains(t, gap.Reason, "missing_app_artifact")
			assert.False(t, gap.Blocking, "a deferred lane reports its gap but must not block")
		})
	}
}

// Visibility and blocking are different things: the executed gate must show the
// gap and still let the release through.
func TestReleaseGateReportsMobileReadinessGapWithoutBlocking(t *testing.T) {
	t.Parallel()

	for _, profile := range []string{"prelaunch", "release-candidate", "postdeploy-smoke"} {
		profile := profile
		t.Run(profile, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, lane := range ReleaseLanes() {
				if lane == "mobile-readiness" || lane == "evidence-dashboard" {
					continue
				}
				writeReleaseJourney(t, dir, lane, lane, releaseTestAdapter(lane), releaseTestArgv(lane))
			}

			payload, err := Execute(Options{
				ProjectDir: dir,
				Profile:    profile,
				Output:     t.TempDir(),
				Runner: LaneRunnerFunc(func(_ Options, lane string) (LaneRunResult, error) {
					return LaneRunResult{
						Status:       LaneStatusPassed,
						RunIndexPath: ".autopus/qa/runs/" + lane + "/run-index.json",
					}, nil
				}),
			})
			require.NoError(t, err)

			row := findLaneRow(t, payload.LaneRows, "mobile-readiness")
			assert.NotEqual(t, SetupGapNone, row.SetupGapClass)
			assert.NotEqual(t, LaneVerdictPass, row.LaneVerdict)
			assert.NotEqual(t, LaneVerdictBlock, row.LaneVerdict)
			assert.Empty(t, payload.Blockers)
			assert.NotEqual(t, GateStatusBlocked, payload.Status)
		})
	}
}

// A lane no adapter can execute must never be advertised as ready, in the
// catalog or in the roadmap that renders from it.
func TestLaneCatalogNeverAdvertisesLaneWithoutExecutorAsReady(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, qalane.WithoutExecutor())
	for _, lane := range qalane.WithoutExecutor() {
		row := laneByID(lane)
		assert.NotEqual(t, "ready", row.ImplementationState, lane)
		assert.Equal(t, "no-adapter-executor", row.ReadinessContract, lane)
		assert.Equal(t, row.ImplementationState, findRoadmapLane(t, Roadmap().Lanes, lane).ImplementationState)
	}
	for _, spec := range SiblingSpecs() {
		for _, lane := range spec.Lanes {
			if !qalane.HasExecutor(lane) {
				assert.NotEqual(t, "ready", spec.Status, spec.SpecID)
			}
		}
	}
}

func findSetupGapRow(t *testing.T, rows []SetupGapRow, lane string) SetupGapRow {
	t.Helper()
	for _, row := range rows {
		if row.Lane == lane {
			return row
		}
	}
	require.Failf(t, "missing setup gap row", "lane=%s", lane)
	return SetupGapRow{}
}

func releaseTestAdapter(lane string) string {
	switch lane {
	case "gui-explore":
		return "gui-explore"
	case "canary-explicit":
		return "canary-template"
	default:
		return "go-test"
	}
}

func releaseTestArgv(lane string) []string {
	switch lane {
	case "gui-explore":
		return []string{"npx", "playwright", "test"}
	case "canary-explicit":
		return []string{"auto", "canary"}
	default:
		return []string{"go", "test", "./..."}
	}
}
