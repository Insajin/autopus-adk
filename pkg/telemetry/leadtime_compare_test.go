package telemetry_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func comparableRun(escaped int, blocked bool, firstSlice time.Duration) telemetry.PipelineRun {
	run := leadRun()
	run.Gates = resolvedSafetyGates()
	if blocked {
		run.Gates = append(run.Gates, telemetry.GateRecord{Gate: "data_loss", Applicability: telemetry.GateApplicabilityBlocked})
	}
	for i := range escaped {
		run.Defects = append(run.Defects, telemetry.DefectRecord{ID: "E" + string(rune('0'+i)), DiscoveredPhase: "release", Escaped: true})
	}
	if firstSlice > 0 {
		run.Milestones = []telemetry.Milestone{{Name: telemetry.MilestoneFirstVerticalSlice, At: leadBase.Add(firstSlice)}}
	}
	return run
}

func TestCompareLeadTime_NoRegressionWhenSafetyDidNotWorsen(t *testing.T) {
	t.Parallel()

	baseline := telemetry.ComputeLeadTime(comparableRun(1, true, 20*time.Minute))
	current := telemetry.ComputeLeadTime(comparableRun(1, false, 15*time.Minute))

	comparison := telemetry.CompareLeadTime(current, baseline)

	assert.False(t, comparison.Regression)
	assert.Empty(t, comparison.RegressionReasons)
	require.NotNil(t, comparison.DeltaFirstSlice)
	assert.Equal(t, -5*time.Minute, *comparison.DeltaFirstSlice)
	assert.Equal(t, -1, comparison.UnresolvedSafetyGatesDelta)
	assert.Equal(t, 1, comparison.BaselineUnresolvedSafetyGates)
}

func TestCompareLeadTime_RegressionOnEscapedDefects(t *testing.T) {
	t.Parallel()

	baseline := telemetry.ComputeLeadTime(comparableRun(0, false, 0))
	current := telemetry.ComputeLeadTime(comparableRun(1, false, 0))

	comparison := telemetry.CompareLeadTime(current, baseline)

	assert.True(t, comparison.Regression)
	assert.Equal(t, []string{"escaped_defects increased 0 -> 1"}, comparison.RegressionReasons)
	assert.Nil(t, comparison.DeltaFirstSlice, "delta is null unless both runs recorded the milestone")
}

func TestCompareLeadTime_RegressionOnUnresolvedSafetyGatesEvenWhenFaster(t *testing.T) {
	t.Parallel()

	baseline := telemetry.ComputeLeadTime(comparableRun(0, false, 0))
	fast := comparableRun(0, true, 0)
	fast.TotalDuration = 10 * time.Minute
	current := telemetry.ComputeLeadTime(fast)

	comparison := telemetry.CompareLeadTime(current, baseline)

	assert.True(t, comparison.Regression)
	assert.Equal(t, []string{"unresolved_safety_gates increased 0 -> 1"}, comparison.RegressionReasons)
	assert.Equal(t, -50*time.Minute, comparison.DeltaCompletion)
}

func TestFormatLeadTime_RendersBodyFreeReport(t *testing.T) {
	t.Parallel()

	run := comparableRun(0, true, 15*time.Minute)
	run.Phases = []telemetry.PhaseRecord{spanPhase("plan", 0, 5), spanPhase("build", 5, 10, "plan")}
	run.Actions = []telemetry.ActionRecord{{Kind: telemetry.ActionKindReread, Target: "/secret/path.go", Reason: "context lost"}}
	report := telemetry.ComputeLeadTime(run)
	comparison := telemetry.CompareLeadTime(report, telemetry.ComputeLeadTime(comparableRun(0, false, 0)))

	text := telemetry.FormatLeadTime(report, &comparison)

	for _, want := range []string{
		"Time to first slice: 15m",
		"Critical path: plan -> build (15m)",
		"Rereads: 1 (context lost: 1)",
		"Unresolved safety gates: data_loss (blocked)",
		"Regression: true",
		"Delta first slice: n/a",
	} {
		assert.Contains(t, text, want)
	}
	assert.False(t, strings.Contains(text, "/secret/path.go"), "action targets never appear in the report")

	plain := telemetry.FormatLeadTime(telemetry.ComputeLeadTime(leadRun()), nil)
	assert.Contains(t, plain, "Time to first slice: n/a")
	assert.NotContains(t, plain, "Baseline")
}
