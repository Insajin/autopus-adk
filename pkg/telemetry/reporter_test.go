package telemetry_test

import (
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

// samplePipelineRun builds a deterministic PipelineRun for reporter tests.
func samplePipelineRun(specID, status, quality string, totalDur time.Duration, retries int) telemetry.PipelineRun {
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	return telemetry.PipelineRun{
		SpecID:        specID,
		StartTime:     now,
		EndTime:       now.Add(totalDur),
		TotalDuration: totalDur,
		Phases: []telemetry.PhaseRecord{
			{
				Name:     "Planning",
				Duration: 45 * time.Second,
				Status:   telemetry.StatusPass,
				Agents: []telemetry.AgentRun{
					{AgentName: "planner", Status: telemetry.StatusPass},
				},
			},
			{
				Name:     "Implementation",
				Duration: 2*time.Minute + 10*time.Second,
				Status:   telemetry.StatusPass,
				Agents: []telemetry.AgentRun{
					{AgentName: "executor", Status: telemetry.StatusPass},
					{AgentName: "executor", Status: telemetry.StatusPass},
				},
			},
		},
		RetryCount:  retries,
		FinalStatus: status,
		QualityMode: quality,
	}
}

// --- FormatSummary ---

func TestFormatSummary_ContainsSpecID(t *testing.T) {
	run := samplePipelineRun("SPEC-TELE-001", telemetry.StatusPass, "balanced", 4*time.Minute+32*time.Second, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "SPEC-TELE-001")
}

func TestFormatSummary_ContainsStatus(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", time.Minute, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "PASS")
}

func TestFormatSummary_ContainsDuration(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", 4*time.Minute+32*time.Second, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "4m 32s")
}

func TestFormatSummary_ContainsQualityMode(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "ultra", time.Minute, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "ultra")
}

func TestFormatSummary_ContainsRetryCount(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", time.Minute, 3)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "3")
}

func TestFormatSummary_PhasesTablePresent(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", 3*time.Minute, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "### Phases")
	assert.Contains(t, out, "Planning")
	assert.Contains(t, out, "Implementation")
}

func TestFormatSummary_AgentMultiplicityCollapsed(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", 3*time.Minute, 0)
	out := telemetry.FormatSummary(run)
	// Two executor agents should appear as "executor×2"
	assert.Contains(t, out, "executor×2")
}

func TestFormatSummary_HasMarkdownHeader(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", time.Minute, 0)
	out := telemetry.FormatSummary(run)
	assert.True(t, strings.HasPrefix(out, "## Pipeline Summary"))
}

// --- FormatComparison ---

func TestFormatComparison_ContainsBothSpecIDs(t *testing.T) {
	run1 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "balanced", 4*time.Minute+32*time.Second, 0)
	run2 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "ultra", 3*time.Minute+15*time.Second, 0)
	out := telemetry.FormatComparison(run1, run2)
	assert.Contains(t, out, "SPEC-001")
	assert.Contains(t, out, "## Pipeline Comparison")
}

func TestFormatComparison_BothDurationsPresent(t *testing.T) {
	run1 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "balanced", 4*time.Minute+32*time.Second, 0)
	run2 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "ultra", 3*time.Minute+15*time.Second, 0)
	out := telemetry.FormatComparison(run1, run2)
	assert.Contains(t, out, "4m 32s")
	assert.Contains(t, out, "3m 15s")
}

func TestFormatComparison_BothQualityModesPresent(t *testing.T) {
	run1 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "balanced", time.Minute, 0)
	run2 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "ultra", time.Minute, 0)
	out := telemetry.FormatComparison(run1, run2)
	assert.Contains(t, out, "balanced")
	assert.Contains(t, out, "ultra")
}

func TestFormatComparison_BothStatusesPresent(t *testing.T) {
	run1 := samplePipelineRun("SPEC-001", telemetry.StatusPass, "balanced", time.Minute, 0)
	run2 := samplePipelineRun("SPEC-001", telemetry.StatusFail, "ultra", time.Minute, 0)
	out := telemetry.FormatComparison(run1, run2)
	assert.Contains(t, out, "PASS")
	assert.Contains(t, out, "FAIL")
}

// --- FormatCostLine ---

func TestFormatCostLine_ContainsDollarAmount(t *testing.T) {
	out := telemetry.FormatCostLine(0.45, "balanced")
	assert.Contains(t, out, "$0.45")
}

func TestFormatCostLine_ContainsQualityMode(t *testing.T) {
	out := telemetry.FormatCostLine(0.45, "balanced")
	assert.Contains(t, out, "Balanced")
}

func TestFormatCostLine_ZeroCost(t *testing.T) {
	out := telemetry.FormatCostLine(0.0, "ultra")
	assert.Contains(t, out, "$0.00")
}

// --- formatDuration (tested indirectly via FormatSummary) ---

func TestFormatSummary_DurationSeconds(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", 45*time.Second, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "45s")
}

func TestFormatSummary_DurationHourMinute(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", 62*time.Minute, 0)
	out := telemetry.FormatSummary(run)
	assert.Contains(t, out, "1h 2m")
}

// --- agentSummary (tested indirectly via FormatSummary) ---

func TestFormatSummary_SingleAgentNoMultiplier(t *testing.T) {
	run := samplePipelineRun("SPEC-X", telemetry.StatusPass, "balanced", time.Minute, 0)
	out := telemetry.FormatSummary(run)
	// planner appears once — no multiplier
	assert.Contains(t, out, "planner")
	assert.NotContains(t, out, "planner×")
}
