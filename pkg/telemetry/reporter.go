// Package telemetry — reporter.go provides human-readable formatting of pipeline run data.
package telemetry

import (
	"fmt"
	"strings"
	"time"
)

// FormatSummary returns a markdown-formatted summary of a single pipeline run.
// Output includes SPEC ID, status, quality mode, duration, retry count, and a
// phases table with per-phase duration, status, and agent summary.
// @AX:ANCHOR: [AUTO] public API — caller in cmd/auto and future CLI commands depend on this signature
func FormatSummary(run PipelineRun) string {
	var b strings.Builder

	b.WriteString("## Pipeline Summary\n\n")
	fmt.Fprintf(&b, "SPEC: %s\n", run.SpecID)
	fmt.Fprintf(&b, "Status: %s\n", run.FinalStatus)
	fmt.Fprintf(&b, "Quality: %s\n", run.QualityMode)
	fmt.Fprintf(&b, "Duration: %s\n", formatDuration(run.TotalDuration))
	fmt.Fprintf(&b, "Retries: %d\n", run.RetryCount)

	if len(run.Phases) > 0 {
		b.WriteString("\n### Phases\n")
		b.WriteString("| Phase | Duration | Status | Agents |\n")
		b.WriteString("|-------|----------|--------|--------|\n")
		for _, p := range run.Phases {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
				p.Name,
				formatDuration(p.Duration),
				p.Status,
				agentSummary(p.Agents),
			)
		}
	}

	return b.String()
}

// FormatComparison returns a markdown table comparing two pipeline runs side by side.
// Rows include SPEC ID, duration, quality mode, status, and retry count.
func FormatComparison(run1, run2 PipelineRun) string {
	var b strings.Builder

	b.WriteString("## Pipeline Comparison\n\n")
	b.WriteString("| | Run 1 | Run 2 |\n")
	b.WriteString("|---|-------|-------|\n")

	rows := []struct {
		label string
		v1    string
		v2    string
	}{
		{"SPEC", run1.SpecID, run2.SpecID},
		{"Duration", formatDuration(run1.TotalDuration), formatDuration(run2.TotalDuration)},
		{"Quality", run1.QualityMode, run2.QualityMode},
		{"Status", run1.FinalStatus, run2.FinalStatus},
		{"Retries", fmt.Sprintf("%d", run1.RetryCount), fmt.Sprintf("%d", run2.RetryCount)},
	}

	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.label, r.v1, r.v2)
	}

	return b.String()
}

// FormatCostLine returns a one-line cost summary for display after pipeline completion.
// qualityMode is title-cased for readability (e.g., "balanced" → "Balanced").
// Example output: "추정 비용: $0.45 (Balanced)"
// @AX:NOTE: [AUTO] output string is Korean — intentional per language policy (user-facing display)
func FormatCostLine(estimatedCost float64, qualityMode string) string {
	label := strings.ToUpper(qualityMode[:1]) + qualityMode[1:]
	return fmt.Sprintf("추정 비용: $%.2f (%s)", estimatedCost, label)
}

// formatDuration converts a time.Duration into a concise human-readable string.
// Examples: 45s → "45s", 4m32s → "4m 32s", 62m → "1h 2m", 3661s → "1h 1m"
// Sub-second durations are rounded up to "0s".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// agentSummary collapses a slice of AgentRun values into a compact string.
// Duplicate agent names are merged with a ×N multiplier.
// Example: [executor, executor, tester] → "executor×2, tester"
func agentSummary(agents []AgentRun) string {
	if len(agents) == 0 {
		return "-"
	}

	// Preserve insertion order while counting occurrences.
	seen := make(map[string]int)
	order := make([]string, 0, len(agents))
	for _, a := range agents {
		if seen[a.AgentName] == 0 {
			order = append(order, a.AgentName)
		}
		seen[a.AgentName]++
	}

	parts := make([]string, 0, len(order))
	for _, name := range order {
		if seen[name] > 1 {
			parts = append(parts, fmt.Sprintf("%s×%d", name, seen[name]))
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}
