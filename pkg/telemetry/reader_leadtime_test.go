package telemetry_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// legacyJSONL is a record written before lead-time fields existed: no
// depends_on, milestones, actions, defects, gates, or estimate anywhere.
const legacyJSONL = `{"type":"pipeline_start","timestamp":"2026-01-10T09:00:00Z","data":{"spec_id":"SPEC-OLD","quality_mode":"balanced"}}
{"type":"phase_start","timestamp":"2026-01-10T09:00:01Z","data":{"name":"implement"}}
{"type":"agent_run","timestamp":"2026-01-10T09:00:02Z","data":{"agent_name":"executor","spec_id":"SPEC-OLD","phase":"implement","start_time":"2026-01-10T09:00:01Z","end_time":"2026-01-10T09:10:01Z","duration_ns":600000000000,"status":"PASS","files_modified":2,"estimated_tokens":100}}
{"type":"phase_end","timestamp":"2026-01-10T09:10:02Z","data":{"status":"PASS"}}
{"type":"pipeline_end","timestamp":"2026-01-10T09:10:03Z","data":{"spec_id":"SPEC-OLD","start_time":"0001-01-01T00:00:00Z","end_time":"2026-01-10T09:10:03Z","total_duration_ns":0,"phases":null,"retry_count":0,"final_status":"PASS","quality_mode":"balanced"}}
`

func writeRawJSONL(t *testing.T, baseDir, content string) {
	t.Helper()
	dir := filepath.Join(baseDir, ".autopus", "telemetry")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "legacy.jsonl"), []byte(content), 0o600))
}

func TestLatestPipelineRun_LegacyRecordLoadsWithoutLeadTimeFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRawJSONL(t, dir, legacyJSONL)

	run, err := telemetry.LatestPipelineRun(dir)
	require.NoError(t, err)
	require.NotNil(t, run)

	assert.Equal(t, "SPEC-OLD", run.SpecID)
	assert.Equal(t, telemetry.StatusPass, run.FinalStatus)
	require.Len(t, run.Phases, 1)
	assert.Equal(t, "implement", run.Phases[0].Name)
	assert.Equal(t, 10*time.Minute, run.Phases[0].Duration)
	assert.Nil(t, run.Phases[0].DependsOn)
	assert.Nil(t, run.Milestones)
	assert.Nil(t, run.Actions)
	assert.Nil(t, run.Defects)
	assert.Nil(t, run.Gates)
	assert.Nil(t, run.Estimate)

	report := telemetry.ComputeLeadTime(*run)
	assert.Equal(t, []string{"implement"}, report.CriticalPath)
	assert.Nil(t, report.TimeToFirstSlice)
	assert.Equal(t, telemetry.SafetyGates, report.UnresolvedSafetyGates)
}

func TestLatestPipelineRun_HydratesLeadTimeEventsAcrossProcesses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Each CLI invocation opens its own short-lived recorder, so every record
	// lands as an event followed by an in-progress pipeline_end marker.
	start, err := telemetry.NewRecorder(dir, "SPEC-CLI")
	require.NoError(t, err)
	start.StartPipeline("SPEC-CLI", "balanced")
	start.StartPhase("plan")
	start.Finalize("")

	for _, phase := range []struct {
		name string
		deps []string
	}{{"plan", nil}, {"build_a", []string{"plan"}}, {"build_b", []string{"plan"}}, {"integration", []string{"build_a", "build_b"}}} {
		rec, err := telemetry.NewRecorder(dir, "SPEC-CLI")
		require.NoError(t, err)
		rec.StartPhase(phase.name, phase.deps...)
		rec.RecordAgent(telemetry.AgentRun{AgentName: "executor", SpecID: "SPEC-CLI", Phase: phase.name, Status: telemetry.StatusPass})
		rec.EndPhase(telemetry.StatusPass)
		rec.Finalize("")
	}

	rec, err := telemetry.NewRecorder(dir, "SPEC-CLI")
	require.NoError(t, err)
	rec.RecordMilestone(telemetry.MilestoneFirstVerticalSlice)
	rec.RecordAction(telemetry.ActionKindReread, "pkg/a.go", "context lost")
	rec.RecordGate(telemetry.GateRecord{Gate: "security", Applicability: telemetry.GateApplicabilityRequired, Resolved: true})
	rec.RecordEstimate(30*time.Minute, 2*time.Hour)
	rec.Finalize("")

	// A concurrent run of another SPEC must not leak into this run.
	other, err := telemetry.NewRecorder(dir, "SPEC-OTHER")
	require.NoError(t, err)
	other.StartPipeline("SPEC-OTHER", "balanced")
	other.RecordDefect(telemetry.DefectRecord{ID: "OTHER-1", DiscoveredPhase: "review"})
	other.Finalize(telemetry.StatusPass)

	end, err := telemetry.NewRecorder(dir, "SPEC-CLI")
	require.NoError(t, err)
	end.Finalize(telemetry.StatusPass)

	// An escaped defect discovered after completion still belongs to the run.
	late, err := telemetry.NewRecorder(dir, "SPEC-CLI")
	require.NoError(t, err)
	late.RecordDefect(telemetry.DefectRecord{ID: "D1", DiscoveredPhase: "integration", FilesTouched: 2, Escaped: true})
	late.Finalize("")

	runs, err := telemetry.PipelineRunsBySpecID(dir, "SPEC-CLI")
	require.NoError(t, err)
	require.Len(t, runs, 1, "only the completed run survives when one exists")
	run := runs[0]

	require.Len(t, run.Phases, 4)
	assert.Equal(t, []string{"plan"}, run.Phases[1].DependsOn)
	assert.Equal(t, []string{"build_a", "build_b"}, run.Phases[3].DependsOn)
	require.Len(t, run.Milestones, 1)
	assert.Equal(t, telemetry.MilestoneFirstVerticalSlice, run.Milestones[0].Name)
	assert.False(t, run.Milestones[0].At.IsZero())
	require.Len(t, run.Actions, 1)
	assert.Equal(t, "context lost", run.Actions[0].Reason)
	require.Len(t, run.Defects, 1)
	assert.Equal(t, "D1", run.Defects[0].ID)
	assert.True(t, run.Defects[0].Escaped)
	require.Len(t, run.Gates, 1)
	assert.True(t, run.Gates[0].Resolved)
	require.NotNil(t, run.Estimate)
	assert.Equal(t, 2*time.Hour, run.Estimate.Max)

	report := telemetry.ComputeLeadTime(run)
	require.NotNil(t, report.TimeToFirstSlice)
	assert.Equal(t, []string{"plan", "build_a", "integration"}, report.CriticalPath)
	assert.Equal(t, 1, report.EscapedDefects)
	assert.Equal(t, []string{"validation", "data_loss", "deterministic_oracle"}, report.UnresolvedSafetyGates)

	others, err := telemetry.PipelineRunsBySpecID(dir, "SPEC-OTHER")
	require.NoError(t, err)
	require.Len(t, others, 1)
	require.Len(t, others[0].Defects, 1)
	assert.Equal(t, "OTHER-1", others[0].Defects[0].ID)
}

func TestLatestPipelineRun_LeadTimeWindowEndsAtNextRunOfSameSpec(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, err := telemetry.NewRecorder(dir, "SPEC-TWICE")
	require.NoError(t, err)
	first.StartPipeline("SPEC-TWICE", "balanced")
	first.RecordGate(telemetry.GateRecord{Gate: "security", Applicability: telemetry.GateApplicabilityBlocked})
	first.Finalize(telemetry.StatusFail)

	second, err := telemetry.NewRecorder(dir, "SPEC-TWICE")
	require.NoError(t, err)
	second.StartPipeline("SPEC-TWICE", "balanced")
	second.RecordGate(telemetry.GateRecord{Gate: "security", Applicability: telemetry.GateApplicabilityRequired, Resolved: true})
	second.Finalize(telemetry.StatusPass)

	runs, err := telemetry.PipelineRunsBySpecID(dir, "SPEC-TWICE")
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Len(t, runs[0].Gates, 1)
	assert.Equal(t, telemetry.GateApplicabilityBlocked, runs[0].Gates[0].Applicability)
	require.Len(t, runs[1].Gates, 1)
	assert.True(t, runs[1].Gates[0].Resolved)
}
