package telemetry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

var pathBase = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

func spanPhase(name string, start, minutes int, deps ...string) telemetry.PhaseRecord {
	begin := pathBase.Add(time.Duration(start) * time.Minute)
	return telemetry.PhaseRecord{
		Name:      name,
		StartTime: begin,
		EndTime:   begin.Add(time.Duration(minutes) * time.Minute),
		DependsOn: deps,
	}
}

func TestComputeLeadTime_ParallelSiblingsAreNotSummed(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		spanPhase("plan", 0, 5),
		spanPhase("build_a", 5, 10, "plan"),
		spanPhase("build_b", 5, 10, "plan"),
	}})

	assert.Equal(t, 15*time.Minute, report.CriticalPathDuration, "two parallel 10m phases after a 5m plan")
	assert.Equal(t, []string{"plan", "build_a"}, report.CriticalPath)
	assert.Empty(t, report.Issues)
}

func TestComputeLeadTime_SerialChainSumsAlongDependencies(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		spanPhase("plan", 0, 5),
		spanPhase("build_a", 5, 10, "plan"),
		spanPhase("build_b", 5, 3, "plan"),
		spanPhase("integration", 15, 4, "build_a", "build_b"),
	}})

	assert.Equal(t, []string{"plan", "build_a", "integration"}, report.CriticalPath)
	assert.Equal(t, 19*time.Minute, report.CriticalPathDuration)
}

func TestComputeLeadTime_UnknownDependencyIsReportedNotFatal(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		spanPhase("plan", 0, 5),
		spanPhase("build", 5, 10, "ghost"),
	}})

	assert.Equal(t, []string{"build"}, report.CriticalPath)
	assert.Equal(t, 10*time.Minute, report.CriticalPathDuration)
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0], `unknown phase "ghost"`)
}

func TestComputeLeadTime_CycleIsReportedNotFatal(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		spanPhase("a", 0, 2, "b"),
		spanPhase("b", 2, 3, "a"),
	}})

	assert.Equal(t, 5*time.Minute, report.CriticalPathDuration)
	require.NotEmpty(t, report.Issues)
	assert.Contains(t, report.Issues[0], "dependency cycle")
}

func TestComputeLeadTime_ZeroDurationTiesPreferLongerChain(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		{Name: "plan"},
		{Name: "build_a", DependsOn: []string{"plan"}},
		{Name: "build_b", DependsOn: []string{"plan"}},
		{Name: "integration", DependsOn: []string{"build_a", "build_b"}},
	}})

	assert.Equal(t, []string{"plan", "build_a", "integration"}, report.CriticalPath)
	assert.Zero(t, report.CriticalPathDuration)
}

func TestComputeLeadTime_DurationFallbackWhenTimestampsMissing(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		{Name: "plan", Duration: 4 * time.Minute},
		{Name: "build", Duration: 6 * time.Minute, DependsOn: []string{"plan"}},
	}})

	assert.Equal(t, 10*time.Minute, report.CriticalPathDuration)
}

func TestComputeLeadTime_RetryResolvesNearestEarlierPhase(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{Phases: []telemetry.PhaseRecord{
		spanPhase("red", 0, 2),
		spanPhase("green", 2, 3, "red"),
		spanPhase("red", 5, 4, "green"),
		spanPhase("refactor", 9, 1, "red"),
	}})

	assert.Equal(t, []string{"red", "green", "red", "refactor"}, report.CriticalPath)
	assert.Equal(t, 10*time.Minute, report.CriticalPathDuration)
	assert.Empty(t, report.Issues)
}

func TestComputeLeadTime_NoPhasesYieldsEmptyPath(t *testing.T) {
	t.Parallel()

	report := telemetry.ComputeLeadTime(telemetry.PipelineRun{})
	assert.Equal(t, []string{}, report.CriticalPath)
	assert.Zero(t, report.CriticalPathDuration)
}
