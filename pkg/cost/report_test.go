// Package cost provides model token pricing and cost estimation utilities.
package cost

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func samplePipelineRun() telemetry.PipelineRun {
	return telemetry.PipelineRun{
		SpecID:      "SPEC-TELE-001",
		QualityMode: "balanced",
		Phases: []telemetry.PhaseRecord{
			{
				Name: "plan",
				Agents: []telemetry.AgentRun{
					{AgentName: "planner", EstimatedTokens: 30_000},
				},
			},
			{
				Name: "execute",
				Agents: []telemetry.AgentRun{
					{AgentName: "executor", EstimatedTokens: 50_000},
				},
			},
		},
	}
}

func TestFormatCostReport_ContainsHeader(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	if !strings.Contains(out, "## Cost Report") {
		t.Errorf("expected '## Cost Report' header, got:\n%s", out)
	}
}

func TestFormatCostReport_ContainsSpecID(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	if !strings.Contains(out, "SPEC-TELE-001") {
		t.Errorf("expected spec ID in output, got:\n%s", out)
	}
}

func TestFormatCostReport_ContainsQualityMode(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	if !strings.Contains(out, "balanced") {
		t.Errorf("expected quality mode in output, got:\n%s", out)
	}
}

func TestFormatCostReport_ContainsTableHeader(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	if !strings.Contains(out, "| Agent") {
		t.Errorf("expected table header in output, got:\n%s", out)
	}
}

func TestFormatCostReport_ContainsAgentRow(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	if !strings.Contains(out, "planner") {
		t.Errorf("expected 'planner' row in output, got:\n%s", out)
	}
	if !strings.Contains(out, "executor") {
		t.Errorf("expected 'executor' row in output, got:\n%s", out)
	}
}

func TestFormatCostReport_ContainsTotalLine(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	if !strings.Contains(out, "**Total:") {
		t.Errorf("expected total line in output, got:\n%s", out)
	}
}

func TestFormatCostReport_ContainsFormattedTokens(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostReport(run)

	// 50,000 should appear with comma separator
	if !strings.Contains(out, "50,000") {
		t.Errorf("expected '50,000' formatted tokens, got:\n%s", out)
	}
}

func TestFormatQualityComparison_ContainsUltra(t *testing.T) {
	out := FormatQualityComparison(100_000)

	if !strings.Contains(out, "Ultra") {
		t.Errorf("expected 'Ultra' in output, got:\n%s", out)
	}
}

func TestFormatQualityComparison_ContainsBalanced(t *testing.T) {
	out := FormatQualityComparison(100_000)

	if !strings.Contains(out, "Balanced") {
		t.Errorf("expected 'Balanced' in output, got:\n%s", out)
	}
}

func TestFormatQualityComparison_ContainsDollarSign(t *testing.T) {
	out := FormatQualityComparison(100_000)

	if !strings.Contains(out, "$") {
		t.Errorf("expected '$' in output, got:\n%s", out)
	}
}

func TestFormatQualityComparison_ContainsTableHeader(t *testing.T) {
	out := FormatQualityComparison(100_000)

	if !strings.Contains(out, "| Mode") {
		t.Errorf("expected '| Mode' table header, got:\n%s", out)
	}
}

func TestFormatCostLine_ContainsKoreanPrefix(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostLine(run)

	if !strings.Contains(out, "추정 비용:") {
		t.Errorf("expected Korean prefix '추정 비용:', got: %s", out)
	}
}

func TestFormatCostLine_ContainsDollarSign(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostLine(run)

	if !strings.Contains(out, "$") {
		t.Errorf("expected '$' in cost line, got: %s", out)
	}
}

func TestFormatCostLine_ContainsQualityMode(t *testing.T) {
	run := samplePipelineRun()
	out := FormatCostLine(run)

	if !strings.Contains(out, "Balanced") {
		t.Errorf("expected 'Balanced' in cost line, got: %s", out)
	}
}

func TestFormatUSD(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{1.36, "$1.36"},
		{0.0, "$0.00"},
		{2.5, "$2.50"},
		{10.123, "$10.12"},
	}

	for _, tc := range tests {
		got := formatUSD(tc.input)
		if got != tc.want {
			t.Errorf("formatUSD(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{50000, "50,000"},
		{1000, "1,000"},
		{999, "999"},
		{1000000, "1,000,000"},
		{0, "0"},
	}

	for _, tc := range tests {
		got := formatTokens(tc.input)
		if got != tc.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
