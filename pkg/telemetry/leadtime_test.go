package telemetry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

var leadBase = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

func leadRun() telemetry.PipelineRun {
	return telemetry.PipelineRun{
		SpecID:        "SPEC-LEAD",
		StartTime:     leadBase,
		EndTime:       leadBase.Add(time.Hour),
		TotalDuration: time.Hour,
	}
}

func resolvedSafetyGates() []telemetry.GateRecord {
	gates := make([]telemetry.GateRecord, 0, len(telemetry.SafetyGates))
	for _, gate := range telemetry.SafetyGates {
		gates = append(gates, telemetry.GateRecord{Gate: gate, Applicability: telemetry.GateApplicabilityRequired, Resolved: true})
	}
	return gates
}

func TestComputeLeadTime_TimeToFirstSlice(t *testing.T) {
	t.Parallel()

	absent := telemetry.ComputeLeadTime(leadRun())
	assert.Nil(t, absent.TimeToFirstSlice)

	run := leadRun()
	run.Milestones = []telemetry.Milestone{
		{Name: "other", At: leadBase.Add(time.Minute)},
		{Name: telemetry.MilestoneFirstVerticalSlice, At: leadBase.Add(20 * time.Minute)},
		{Name: telemetry.MilestoneFirstVerticalSlice, At: leadBase.Add(12 * time.Minute)},
	}
	present := telemetry.ComputeLeadTime(run)
	require.NotNil(t, present.TimeToFirstSlice)
	assert.Equal(t, 12*time.Minute, *present.TimeToFirstSlice, "earliest first_vertical_slice wins")

	run.StartTime = time.Time{}
	unknownStart := telemetry.ComputeLeadTime(run)
	assert.Nil(t, unknownStart.TimeToFirstSlice)
	assert.Contains(t, unknownStart.Issues, "first_vertical_slice recorded but run start time is unknown")

	run.StartTime = leadBase.Add(30 * time.Minute)
	beforeStart := telemetry.ComputeLeadTime(run)
	assert.Nil(t, beforeStart.TimeToFirstSlice)
	assert.Contains(t, beforeStart.Issues, "first_vertical_slice precedes run start")
}

func TestComputeLeadTime_ActionsGroupedByReason(t *testing.T) {
	t.Parallel()

	run := leadRun()
	run.Actions = []telemetry.ActionRecord{
		{Kind: telemetry.ActionKindReread, Target: "a.go", Reason: "context lost"},
		{Kind: telemetry.ActionKindReread, Target: "b.go", Reason: "context lost"},
		{Kind: telemetry.ActionKindReread, Target: "c.go"},
		{Kind: telemetry.ActionKindRerun, Target: "go test", Reason: "flaky"},
		{Kind: "bogus", Target: "x"},
	}
	report := telemetry.ComputeLeadTime(run)

	assert.Equal(t, 3, report.RereadCount)
	assert.Equal(t, 1, report.RerunCount)
	assert.Equal(t, map[string]int{"context lost": 2, "unspecified": 1}, report.Rereads)
	assert.Equal(t, map[string]int{"flaky": 1}, report.Reruns)
	assert.Contains(t, report.Issues, `unknown action kind "bogus" ignored`)
}

func TestComputeLeadTime_DefectsByPhaseDedupeByID(t *testing.T) {
	t.Parallel()

	run := leadRun()
	run.Defects = []telemetry.DefectRecord{
		{ID: "D1", DiscoveredPhase: "integration", FilesTouched: 2},
		{ID: "D1", DiscoveredPhase: "integration", FixedPhase: "implement", FilesTouched: 3},
		{ID: "D2", DiscoveredPhase: "review", FilesTouched: 1, Repeat: true},
		{ID: "D3", DiscoveredPhase: "", Escaped: true},
		{DiscoveredPhase: "review"},
		{DiscoveredPhase: "review"},
	}
	report := telemetry.ComputeLeadTime(run)

	assert.Equal(t, map[string]telemetry.PhaseDefects{
		"integration": {Count: 1, FilesTouched: 3},
		"review":      {Count: 3, FilesTouched: 1},
		"unknown":     {Count: 1},
	}, report.DefectsByDiscoveryPhase)
	assert.Equal(t, 1, report.EscapedDefects)
	assert.InDelta(t, 0.2, report.RepeatFindingRate, 1e-9, "1 repeat of 5 distinct defects")
}

func TestComputeLeadTime_NoDefectsHasZeroRepeatRate(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(leadRun())
	assert.Zero(t, report.RepeatFindingRate)
	assert.Empty(t, report.DefectsByDiscoveryPhase)
}

func TestComputeLeadTime_SafetyGates(t *testing.T) {
	t.Parallel()

	omitted := telemetry.ComputeLeadTime(leadRun())
	assert.Equal(t, telemetry.SafetyGates, omitted.UnresolvedSafetyGates, "never-recorded safety gates are omissions")
	assert.Equal(t, telemetry.SafetyGateOmitted, omitted.SafetyGateStatus["security"])

	run := leadRun()
	run.Gates = append(resolvedSafetyGates(),
		telemetry.GateRecord{Gate: "data_loss", Applicability: telemetry.GateApplicabilityBlocked, Resolved: true},
		telemetry.GateRecord{Gate: "validation", Applicability: telemetry.GateApplicabilityReusable},
		telemetry.GateRecord{Gate: "deterministic_oracle", Applicability: telemetry.GateApplicabilityNotApplicable, Resolved: true},
		telemetry.GateRecord{Gate: "accessibility", Applicability: telemetry.GateApplicabilityNotApplicable},
	)
	report := telemetry.ComputeLeadTime(run)

	assert.Equal(t, []string{"validation", "data_loss", "deterministic_oracle"}, report.UnresolvedSafetyGates)
	assert.Equal(t, map[string]string{
		"security":             telemetry.SafetyGateResolved,
		"validation":           telemetry.SafetyGateUnresolved,
		"data_loss":            telemetry.SafetyGateBlocked,
		"deterministic_oracle": telemetry.SafetyGateNotApplicable,
	}, report.SafetyGateStatus, "latest record per gate wins; blocked and not_applicable never count as resolved")

	run.Gates = append(run.Gates, telemetry.GateRecord{Gate: "data_loss", Applicability: telemetry.GateApplicabilityRequired, Resolved: true})
	fixed := telemetry.ComputeLeadTime(run)
	assert.NotContains(t, fixed.UnresolvedSafetyGates, "data_loss")
}

func TestComputeLeadTime_EstimateVsActual(t *testing.T) {
	t.Parallel()

	none := telemetry.ComputeLeadTime(leadRun())
	assert.Nil(t, none.EstimateVsActual)

	run := leadRun()
	run.Estimate = &telemetry.EstimateRecord{Min: 30 * time.Minute, Max: 2 * time.Hour}
	within := telemetry.ComputeLeadTime(run)
	require.NotNil(t, within.EstimateVsActual)
	assert.Equal(t, telemetry.EstimateVsActual{Min: 30 * time.Minute, Max: 2 * time.Hour, Actual: time.Hour, WithinRange: true}, *within.EstimateVsActual)

	run.Estimate = &telemetry.EstimateRecord{Min: 10 * time.Minute, Max: 59 * time.Minute}
	outside := telemetry.ComputeLeadTime(run)
	assert.False(t, outside.EstimateVsActual.WithinRange)

	run.Estimate = &telemetry.EstimateRecord{Min: time.Hour, Max: time.Hour}
	boundary := telemetry.ComputeLeadTime(run)
	assert.True(t, boundary.EstimateVsActual.WithinRange, "range bounds are inclusive")
}

func TestComputeLeadTime_CompletionFallsBackToTimestamps(t *testing.T) {
	t.Parallel()

	run := leadRun()
	run.TotalDuration = 0
	report := telemetry.ComputeLeadTime(run)
	assert.Equal(t, time.Hour, report.CompletionLeadTime)
}
