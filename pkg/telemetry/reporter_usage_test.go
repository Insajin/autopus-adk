package telemetry_test

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func TestFormatUsageCost_ActualEstimatedAndCallsAreSeparated(t *testing.T) {
	t.Parallel()

	actual := reporterUsage("actual", "task", 100, 20, 0.02)
	estimated := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "estimated", CallID: "call", TaskID: "task",
		Source: telemetry.UsageSourceEstimate, EstimatedTotalTokens: reporterInt64(75),
		EstimatedCostUSD: reporterFloat64(0.01),
	})
	run := reporterRun(telemetry.AgentRun{
		AgentName: "executor", TaskID: "task", Status: telemetry.StatusPass,
		AcceptanceStatus: telemetry.StatusPass, ToolCalls: 3,
		Usage: []telemetry.UsageEnvelope{actual, estimated},
	})

	got := telemetry.FormatUsageCost(run)
	for _, want := range []string{"실제 비용: $0.0200", "추정 비용: $0.0100", "모델 호출: 2", "도구 호출: 3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatUsageCost() = %q, want %q", got, want)
		}
	}
}

func TestFormatUsageCost_UnobservedRunIsEmptyAndMissingCostIsNull(t *testing.T) {
	t.Parallel()

	if got := telemetry.FormatUsageCost(telemetry.PipelineRun{}); got != "" {
		t.Fatalf("unobserved FormatUsageCost() = %q, want empty", got)
	}

	run := reporterRun(telemetry.AgentRun{AgentName: "executor", ToolCalls: 1})
	got := telemetry.FormatUsageCost(run)
	if got != "실제 비용: null | 추정 비용: null | 모델 호출: 0 | 도구 호출: 1\n" {
		t.Fatalf("missing-cost FormatUsageCost() = %q", got)
	}
}

func TestFormatComparison_UsageRowsUseRawAndBillableOracles(t *testing.T) {
	t.Parallel()

	cold := reporterRun(reporterAcceptedAgent("cold", 1000, 200, 0.020))
	warm := reporterRun(reporterAcceptedAgent("warm", 1000, 200, 0.014))
	got := telemetry.FormatComparison(cold, warm)

	for _, want := range []string{
		"| Raw tokens | 1200 | 1200 |",
		"| Actual coverage | 100.0% | 100.0% |",
		"| Accepted tasks | 1 | 1 |",
		"| Raw / accepted | 1200.00 | 1200.00 |",
		"| Actual cost | $0.0200 | $0.0140 |",
		"| Raw reduction |  | 0.000% |",
		"| Actual cost reduction |  | 30.000% |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatComparison() missing %q:\n%s", want, got)
		}
	}
}

func TestFormatComparison_OneUnobservedSideRendersNullableFields(t *testing.T) {
	t.Parallel()

	left := telemetry.PipelineRun{SpecID: "left"}
	right := reporterRun(reporterAcceptedAgent("right", 10, 5, 0))
	got := telemetry.FormatComparison(left, right)

	for _, want := range []string{
		"| Raw / accepted | null | 15.00 |",
		"| Actual cost | null | $0.0000 |",
		"| Actual cost reduction |  | null |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatComparison() missing nullable row %q:\n%s", want, got)
		}
	}
}

func reporterRun(agent telemetry.AgentRun) telemetry.PipelineRun {
	return telemetry.PipelineRun{
		SpecID: "SPEC-REPORT", FinalStatus: telemetry.StatusPass, QualityMode: "ultra",
		Phases: []telemetry.PhaseRecord{{Name: "implementation", Agents: []telemetry.AgentRun{agent}}},
	}
}

func reporterAcceptedAgent(runID string, input, output int64, cost float64) telemetry.AgentRun {
	return telemetry.AgentRun{
		AgentName: "executor", TaskID: "task", Status: telemetry.StatusPass,
		AcceptanceStatus: telemetry.StatusPass,
		Usage:            []telemetry.UsageEnvelope{reporterUsage(runID, "task", input, output, cost)},
	}
}

func reporterUsage(runID, taskID string, input, output int64, cost float64) telemetry.UsageEnvelope {
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: "call", TaskID: taskID, Source: telemetry.UsageSourceProvider,
		InputTokensTotal: reporterInt64(input), OutputTokensTotal: reporterInt64(output),
		ActualCostUSD: reporterFloat64(cost),
	})
}

func reporterInt64(value int64) *int64       { return &value }
func reporterFloat64(value float64) *float64 { return &value }
