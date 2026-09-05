package telemetry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func TestRecorder_LeadTimeRecordsRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec, err := telemetry.NewRecorder(dir, "SPEC-LEAD-REC")
	require.NoError(t, err)

	rec.StartPipeline("SPEC-LEAD-REC", "balanced")
	rec.StartPhase("plan")
	rec.EndPhase(telemetry.StatusPass)
	rec.StartPhase("build", "plan", " plan ", "", "plan")
	rec.EndPhase(telemetry.StatusPass)
	rec.RecordMilestone(telemetry.MilestoneFirstVerticalSlice)
	rec.RecordAction(telemetry.ActionKindRerun, "go test ./...", "flaky")
	rec.RecordDefect(telemetry.DefectRecord{ID: "D1", DiscoveredPhase: "build", FilesTouched: 1, Repeat: true})
	rec.RecordGate(telemetry.GateRecord{Gate: "security", Applicability: telemetry.GateApplicabilityRequired, Resolved: true})
	rec.RecordEstimate(time.Hour, 3*time.Hour)
	rec.RecordEstimate(2*time.Hour, 4*time.Hour)
	returned := rec.Finalize(telemetry.StatusPass)

	assert.Equal(t, []string{"plan"}, returned.Phases[1].DependsOn, "dependencies are trimmed and de-duplicated")
	assert.Nil(t, returned.Phases[0].DependsOn)
	require.Len(t, returned.Milestones, 1)
	assert.False(t, returned.Milestones[0].At.IsZero())
	require.Len(t, returned.Defects, 1)
	assert.False(t, returned.Defects[0].At.IsZero(), "At defaults to the recording time")
	require.NotNil(t, returned.Estimate)
	assert.Equal(t, telemetry.EstimateRecord{Min: 2 * time.Hour, Max: 4 * time.Hour}, *returned.Estimate, "a later estimate replaces the earlier one")

	loaded, err := telemetry.LatestPipelineRun(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, []string{"plan"}, loaded.Phases[1].DependsOn)
	require.Len(t, loaded.Milestones, 1)
	assert.True(t, loaded.Milestones[0].At.Equal(returned.Milestones[0].At))
	require.Len(t, loaded.Actions, 1)
	assert.Equal(t, telemetry.ActionKindRerun, loaded.Actions[0].Kind)
	require.Len(t, loaded.Defects, 1)
	assert.True(t, loaded.Defects[0].Repeat)
	require.Len(t, loaded.Gates, 1)
	assert.Equal(t, "security", loaded.Gates[0].Gate)
	require.NotNil(t, loaded.Estimate)
	assert.Equal(t, *returned.Estimate, *loaded.Estimate)
}

func TestValidateGateApplicability(t *testing.T) {
	t.Parallel()

	assert.NoError(t, telemetry.ValidateGateApplicability("security", telemetry.GateApplicabilityBlocked))
	assert.NoError(t, telemetry.ValidateGateApplicability("accessibility", telemetry.GateApplicabilityNotApplicable))
	assert.Error(t, telemetry.ValidateGateApplicability("security", telemetry.GateApplicabilityNotApplicable), "safety gates are never not_applicable")
	assert.Error(t, telemetry.ValidateGateApplicability("build", "maybe"))
}

func TestValidateEstimate(t *testing.T) {
	t.Parallel()

	assert.NoError(t, telemetry.ValidateEstimate(time.Minute, time.Minute))
	assert.Error(t, telemetry.ValidateEstimate(2*time.Minute, time.Minute))
	assert.Error(t, telemetry.ValidateEstimate(0, time.Minute))
}
