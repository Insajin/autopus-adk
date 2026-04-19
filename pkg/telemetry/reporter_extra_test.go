package telemetry_test

import (
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

// TestFormatSummary_NoPhases_OmitsTable verifies that a run with no phases
// does not render the "### Phases" section.
func TestFormatSummary_NoPhases_OmitsTable(t *testing.T) {
	run := telemetry.PipelineRun{
		SpecID:        "SPEC-EMPTY",
		FinalStatus:   telemetry.StatusPass,
		QualityMode:   "balanced",
		TotalDuration: 10 * time.Second,
	}
	out := telemetry.FormatSummary(run)
	assert.NotContains(t, out, "### Phases")
}

// TestFormatSummary_ZeroDuration verifies that a zero-duration run renders "0s".
func TestFormatSummary_ZeroDuration(t *testing.T) {
	run := telemetry.PipelineRun{
		SpecID:        "SPEC-ZERO",
		FinalStatus:   telemetry.StatusPass,
		QualityMode:   "balanced",
		TotalDuration: 0,
	}
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "0s")
}

// TestFormatSummary_DurationExactlyHour verifies the "Xh" branch (minutes == 0).
func TestFormatSummary_DurationExactlyHour(t *testing.T) {
	run := telemetry.PipelineRun{
		SpecID:        "SPEC-HOUR",
		FinalStatus:   telemetry.StatusPass,
		QualityMode:   "ultra",
		TotalDuration: 2 * time.Hour,
	}
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "2h")
	assert.NotContains(t, out, "2h 0m")
}

// TestFormatSummary_DurationExactlyMinutes verifies the "Xm" branch (seconds == 0).
func TestFormatSummary_DurationExactlyMinutes(t *testing.T) {
	run := telemetry.PipelineRun{
		SpecID:        "SPEC-MINS",
		FinalStatus:   telemetry.StatusPass,
		QualityMode:   "balanced",
		TotalDuration: 3 * time.Minute,
	}
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "3m")
	assert.NotContains(t, out, "3m 0s")
}

// TestFormatSummary_LongAgentName exercises agentSummary with an unusually
// long agent name to confirm no truncation or panic occurs.
func TestFormatSummary_LongAgentName(t *testing.T) {
	longName := "a-very-long-agent-name-that-exceeds-normal-length"
	run := telemetry.PipelineRun{
		SpecID:        "SPEC-LONG",
		FinalStatus:   telemetry.StatusPass,
		QualityMode:   "balanced",
		TotalDuration: time.Minute,
		Phases: []telemetry.PhaseRecord{
			{
				Name:     "Planning",
				Duration: time.Second,
				Status:   telemetry.StatusPass,
				Agents: []telemetry.AgentRun{
					{AgentName: longName, Status: telemetry.StatusPass},
				},
			},
		},
	}
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, longName)
	assert.NotContains(t, out, longName+"×")
}

// TestFormatSummary_EmptyPhaseAgents verifies that a phase with no agents
// renders a dash in the agents column.
func TestFormatSummary_EmptyPhaseAgents(t *testing.T) {
	run := telemetry.PipelineRun{
		SpecID:        "SPEC-NOAG",
		FinalStatus:   telemetry.StatusPass,
		QualityMode:   "balanced",
		TotalDuration: 30 * time.Second,
		Phases: []telemetry.PhaseRecord{
			{
				Name:     "Planning",
				Duration: 10 * time.Second,
				Status:   telemetry.StatusPass,
				Agents:   nil,
			},
		},
	}
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "| Planning |")
	// agentSummary(nil) returns "-"
	assert.Contains(t, out, "| - |")
}
