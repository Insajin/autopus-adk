package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// seedLeadTimeRun records one complete CLI-style run: each call is what a
// separate `auto telemetry record` process would do.
func seedLeadTimeRun(t *testing.T, dir, spec string, dataLossResolved, escaped bool) {
	t.Helper()
	record := func(p recordParams) {
		t.Helper()
		p.specID = spec
		require.NoError(t, runTelemetryRecord(dir, p), "action %s", p.action)
	}
	record(recordParams{action: "start", phase: "plan", qualityMode: "balanced"})
	record(recordParams{action: "estimate", estimateMin: 30 * time.Minute, estimateMax: 2 * time.Hour})
	record(recordParams{action: "agent", agent: "planner", phase: "plan", status: "PASS"})
	record(recordParams{action: "agent", agent: "executor", phase: "build_a", dependsOn: []string{"plan"}, status: "PASS"})
	record(recordParams{action: "agent", agent: "executor", phase: "build_b", dependsOn: []string{"plan"}, status: "PASS"})
	record(recordParams{action: "milestone", name: telemetry.MilestoneFirstVerticalSlice})
	record(recordParams{action: "agent", agent: "tester", phase: "integration", dependsOn: []string{"build_a", "build_b"}, status: "PASS"})
	record(recordParams{action: "action", kind: "reread", target: "pkg/a.go", reason: "context lost"})
	record(recordParams{action: "defect", defectID: "D1", discoveredPhase: "integration", files: 2, escaped: escaped})
	record(recordParams{action: "gate", gate: "security", applicability: "required", resolved: true})
	record(recordParams{action: "gate", gate: "validation", applicability: "required", resolved: true})
	record(recordParams{action: "gate", gate: "deterministic_oracle", applicability: "reusable", resolved: true})
	if dataLossResolved {
		record(recordParams{action: "gate", gate: "data_loss", applicability: "required", resolved: true})
	} else {
		record(recordParams{action: "gate", gate: "data_loss", applicability: "blocked"})
	}
	record(recordParams{action: "end", status: "PASS"})
}

func TestTelemetryRecordLeadTimeSubrecordsRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT", false, true)

	run, err := telemetry.LatestPipelineRun(dir)
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "PASS", run.FinalStatus)

	require.Len(t, run.Phases, 4)
	assert.Equal(t, []string{"plan"}, run.Phases[1].DependsOn)
	assert.Equal(t, []string{"build_a", "build_b"}, run.Phases[3].DependsOn)
	require.Len(t, run.Milestones, 1)
	require.Len(t, run.Actions, 1)
	assert.Equal(t, "reread", run.Actions[0].Kind)
	require.Len(t, run.Defects, 1)
	assert.Equal(t, 2, run.Defects[0].FilesTouched)
	assert.True(t, run.Defects[0].Escaped)
	require.Len(t, run.Gates, 4)
	require.NotNil(t, run.Estimate)
	assert.Equal(t, 30*time.Minute, run.Estimate.Min)

	report := telemetry.ComputeLeadTime(*run)
	require.NotNil(t, report.TimeToFirstSlice)
	assert.Equal(t, []string{"plan", "build_a", "integration"}, report.CriticalPath)
	assert.Equal(t, map[string]int{"context lost": 1}, report.Rereads)
	assert.Equal(t, 1, report.EscapedDefects)
	assert.Equal(t, []string{"data_loss"}, report.UnresolvedSafetyGates)
	assert.Equal(t, telemetry.SafetyGateBlocked, report.SafetyGateStatus["data_loss"])
}

func TestTelemetryRecordLeadTimeValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tests := []struct {
		name string
		p    recordParams
		want string
	}{
		{"milestone without name", recordParams{specID: "S", action: "milestone"}, "--name is required"},
		{"action bad kind", recordParams{specID: "S", action: "action", kind: "skim", target: "x", reason: "r"}, `unknown action kind "skim"`},
		{"action without target", recordParams{specID: "S", action: "action", kind: "rerun", reason: "r"}, "--target is required"},
		{"action without reason", recordParams{specID: "S", action: "action", kind: "rerun", target: "x"}, "--reason is required"},
		{"depends-on without phase", recordParams{specID: "S", action: "agent", agent: "a", dependsOn: []string{"plan"}}, "--depends-on requires --phase"},
		{"depends-on on subrecord", recordParams{specID: "S", action: "milestone", name: "m", phase: "p", dependsOn: []string{"plan"}}, "--depends-on requires --phase"},
		{"defect without id", recordParams{specID: "S", action: "defect", discoveredPhase: "p"}, "--id is required"},
		{"defect without phase", recordParams{specID: "S", action: "defect", defectID: "D"}, "--discovered-phase is required"},
		{"defect negative files", recordParams{specID: "S", action: "defect", defectID: "D", discoveredPhase: "p", files: -1}, "--files must not be negative"},
		{"gate without id", recordParams{specID: "S", action: "gate", applicability: "required"}, "--gate is required"},
		{"gate bad applicability", recordParams{specID: "S", action: "gate", gate: "build", applicability: "maybe"}, `unknown gate applicability "maybe"`},
		{"safety gate not applicable", recordParams{specID: "S", action: "gate", gate: "security", applicability: "not_applicable"}, `safety gate "security" cannot be not_applicable`},
		{"estimate inverted", recordParams{specID: "S", action: "estimate", estimateMin: time.Hour, estimateMax: time.Minute}, "exceeds max"},
		{"estimate missing", recordParams{specID: "S", action: "estimate"}, "must be positive"},
		{"unknown action lists subrecords", recordParams{specID: "S", action: "bogus"}, "milestone|action|defect|gate|estimate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runTelemetryRecord(dir, test.p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
	for _, action := range []string{"milestone", "action", "defect", "gate", "estimate"} {
		err := runTelemetryRecord(dir, recordParams{action: action, name: "n", kind: "reread", target: "t", reason: "r",
			defectID: "D", discoveredPhase: "p", gate: "build", applicability: "required", estimateMin: time.Minute, estimateMax: time.Hour})
		require.Error(t, err, action)
		assert.Contains(t, err.Error(), "--spec-id is required", action)
	}

	run, err := telemetry.LatestPipelineRun(dir)
	require.NoError(t, err)
	assert.Nil(t, run, "rejected records must not create telemetry")
}
