package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func sampleTelemetryRun(status string) telemetry.PipelineRun {
	return telemetry.PipelineRun{
		SpecID:      "SPEC-T-001",
		FinalStatus: status,
		QualityMode: "fast",
		Phases: []telemetry.PhaseRecord{
			{
				Name: "implement",
				Agents: []telemetry.AgentRun{
					{AgentName: "executor", Status: "PASS", EstimatedTokens: 1000, Duration: 5 * time.Second},
					{AgentName: "tester", Status: "PASS", EstimatedTokens: 500, Duration: 2 * time.Second},
				},
			},
		},
	}
}

// TestBuildTelemetrySummaryPayload_WrapsRun verifies the run is embedded as-is.
func TestBuildTelemetrySummaryPayload_WrapsRun(t *testing.T) {
	t.Parallel()

	run := sampleTelemetryRun(telemetry.StatusPass)
	payload := buildTelemetrySummaryPayload(run)
	assert.Equal(t, "SPEC-T-001", payload.Run.SpecID)
	assert.Equal(t, telemetry.StatusPass, payload.Run.FinalStatus)
}

// TestBuildTelemetryCostPayload_AggregatesAgents verifies per-agent rows and totals.
func TestBuildTelemetryCostPayload_AggregatesAgents(t *testing.T) {
	t.Parallel()

	run := sampleTelemetryRun(telemetry.StatusPass)
	payload := buildTelemetryCostPayload(run)
	require.Len(t, payload.Agents, 2)
	assert.Equal(t, "implement", payload.Agents[0].Phase)
	assert.Equal(t, "executor", payload.Agents[0].Agent)
	assert.Equal(t, 1000, payload.Agents[0].Tokens)
	assert.Equal(t, "PASS", payload.Agents[0].Status)
	assert.Equal(t, int64(5*time.Second), payload.Agents[0].Duration)
	// Total cost is the sum of per-agent estimates and must be non-negative.
	assert.GreaterOrEqual(t, payload.TotalCostUSD, 0.0)
}

// TestBuildTelemetryCostPayload_NoPhases yields empty agent slice.
func TestBuildTelemetryCostPayload_NoPhases(t *testing.T) {
	t.Parallel()

	run := telemetry.PipelineRun{QualityMode: "fast"}
	payload := buildTelemetryCostPayload(run)
	assert.Empty(t, payload.Agents)
}

// TestBuildTelemetryComparePayload_WrapsRuns embeds all runs.
func TestBuildTelemetryComparePayload_WrapsRuns(t *testing.T) {
	t.Parallel()

	runs := []telemetry.PipelineRun{sampleTelemetryRun("PASS"), sampleTelemetryRun("FAIL")}
	payload := buildTelemetryComparePayload(runs)
	require.Len(t, payload.Runs, 2)
	assert.Equal(t, "FAIL", payload.Runs[1].FinalStatus)
}

// TestBuildTelemetryRunWarnings_Pass returns nil for PASS runs.
func TestBuildTelemetryRunWarnings_Pass(t *testing.T) {
	t.Parallel()

	assert.Nil(t, buildTelemetryRunWarnings(sampleTelemetryRun(telemetry.StatusPass)))
}

// TestBuildTelemetryRunWarnings_Fail flags a non-PASS run.
func TestBuildTelemetryRunWarnings_Fail(t *testing.T) {
	t.Parallel()

	warnings := buildTelemetryRunWarnings(sampleTelemetryRun(telemetry.StatusFail))
	require.Len(t, warnings, 1)
	assert.Equal(t, "pipeline_not_pass", warnings[0].Code)
}

// TestBuildTelemetryComparisonWarnings_AllPass returns no warnings.
func TestBuildTelemetryComparisonWarnings_AllPass(t *testing.T) {
	t.Parallel()

	runs := []telemetry.PipelineRun{sampleTelemetryRun("PASS"), sampleTelemetryRun("PASS")}
	assert.Empty(t, buildTelemetryComparisonWarnings(runs))
}

// TestBuildTelemetryComparisonWarnings_OneFail flags a single non-PASS member.
func TestBuildTelemetryComparisonWarnings_OneFail(t *testing.T) {
	t.Parallel()

	runs := []telemetry.PipelineRun{sampleTelemetryRun("PASS"), sampleTelemetryRun("FAIL")}
	warnings := buildTelemetryComparisonWarnings(runs)
	require.Len(t, warnings, 1)
	assert.Equal(t, "pipeline_not_pass", warnings[0].Code)
}
