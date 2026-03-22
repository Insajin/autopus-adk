// Package cost provides model token pricing and cost estimation utilities.
package cost

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// FormatCostReport produces a Markdown cost breakdown table for all agents in a pipeline run.
func FormatCostReport(run telemetry.PipelineRun) string {
	est := NewEstimator(run.QualityMode)
	var sb strings.Builder

	sb.WriteString("## Cost Report\n\n")
	fmt.Fprintf(&sb, "SPEC: %s\n", run.SpecID)
	fmt.Fprintf(&sb, "Quality: %s\n\n", run.QualityMode)

	sb.WriteString("| Agent | Model | Tokens | Cost |\n")
	sb.WriteString("|-------|-------|--------|------|\n")

	var total float64
	for _, phase := range run.Phases {
		for _, agent := range phase.Agents {
			model := ModelForAgent(run.QualityMode, agent.AgentName)
			cost := est.EstimateCost(agent)
			total += cost
			fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n",
				agent.AgentName,
				model,
				formatTokens(agent.EstimatedTokens),
				formatUSD(cost),
			)
		}
	}

	fmt.Fprintf(&sb, "\n**Total: %s**\n", formatUSD(total))
	return sb.String()
}

// FormatQualityComparison produces a Markdown table comparing ultra vs balanced mode costs.
// Used by the quality selection UI (R7) before pipeline execution.
func FormatQualityComparison(totalTokens int) string {
	est := NewEstimator("ultra")
	ultraCost, balancedCost := est.EstimateQualityComparison(totalTokens)

	var sb strings.Builder
	sb.WriteString("| Mode | Estimated Cost |\n")
	sb.WriteString("|------|---------------|\n")
	fmt.Fprintf(&sb, "| Ultra | %s |\n", formatUSD(ultraCost))
	fmt.Fprintf(&sb, "| Balanced | %s |\n", formatUSD(balancedCost))
	return sb.String()
}

// FormatCostLine returns a single-line cost summary for inline display.
// Example: "추정 비용: $0.45 (Balanced)"
func FormatCostLine(run telemetry.PipelineRun) string {
	est := NewEstimator(run.QualityMode)
	total := est.EstimatePipelineCost(run)
	mode := strings.ToUpper(run.QualityMode[:1]) + run.QualityMode[1:]
	return fmt.Sprintf("추정 비용: %s (%s)", formatUSD(total), mode)
}

// formatUSD formats a float64 as a USD string with two decimal places.
// Example: 1.36 → "$1.36"
func formatUSD(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

// formatTokens formats an integer with comma separators.
// Example: 50000 → "50,000"
func formatTokens(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Insert commas from right to left every three digits.
	var result []byte
	for i, ch := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}
