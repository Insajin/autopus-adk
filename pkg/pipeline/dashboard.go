// Package pipeline provides pipeline state management types and persistence.
package pipeline

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// PhaseStatus represents the display status of a pipeline phase.
type PhaseStatus string

const (
	// PhasePending indicates the phase has not started.
	PhasePending PhaseStatus = "pending"
	// PhaseRunning indicates the phase is currently executing.
	PhaseRunning PhaseStatus = "running"
	// PhaseDone indicates the phase completed successfully.
	PhaseDone PhaseStatus = "done"
	// PhaseFailed indicates the phase encountered an error.
	PhaseFailed PhaseStatus = "failed"
	// PhaseSkipped indicates the phase was intentionally not dispatched.
	PhaseSkipped PhaseStatus = "skipped"
	// PhaseCancelled indicates execution was cancelled before completion.
	PhaseCancelled PhaseStatus = "cancelled"
)

// DashboardData holds all data needed to render a pipeline dashboard.
type DashboardData struct {
	Phases  map[string]PhaseStatus
	Agents  map[string]string // agent name -> status
	Blocker string
	Elapsed time.Duration
}

// @AX:NOTE [AUTO] @AX:REASON: magic constants — phase keys and display names define canonical rendering order; changes here must be coordinated with pipeline_dashboard.go phase map
var phaseOrder = []struct {
	key  string
	name string
}{
	{string(PhasePlan), "Planning"},
	{string(PhaseTestScaffold), "Test Scaffold"},
	{string(PhaseImplement), "Implementation"},
	{string(PhaseValidate), "Validation"},
	{string(PhaseReview), "Review"},
	// Legacy keys remain renderable for pre-v1 in-memory callers.
	{"phase1", "Planning"},
	{"phase1.5", "Test Scaffold"},
	{"phase2", "Implementation"},
	{"phase3", "Testing"},
	{"phase4", "Review"},
}

// @AX:NOTE [AUTO] @AX:REASON: magic constant — box width for dashboard rendering; changing affects all box-drawing alignment
const boxWidth = 38

// statusIcon returns a display icon for the given phase status.
func statusIcon(s PhaseStatus) string {
	switch s {
	case PhaseDone:
		return "\033[32m✓ done\033[0m"
	case PhaseRunning:
		return "\033[33m▶ running\033[0m"
	case PhaseFailed:
		return "\033[31m✗ failed\033[0m"
	case PhaseSkipped:
		return "\033[36m↷ skipped\033[0m"
	case PhaseCancelled:
		return "\033[31m■ cancelled\033[0m"
	default:
		return "\033[2m○ pending\033[0m"
	}
}

// RenderDashboard produces a box-drawing string representation of the pipeline dashboard.
func RenderDashboard(data DashboardData) string {
	var b strings.Builder

	top := "╔" + strings.Repeat("═", boxWidth) + "╗"
	sep := "╠" + strings.Repeat("═", boxWidth) + "╣"
	bot := "╚" + strings.Repeat("═", boxWidth) + "╝"

	b.WriteString(top + "\n")
	b.WriteString(boxLine("Pipeline Dashboard") + "\n")
	b.WriteString(sep + "\n")

	// Render phases in fixed order.
	for _, p := range phaseOrder {
		status, ok := data.Phases[p.key]
		if !ok {
			continue
		}
		label := fmt.Sprintf("%s: %s", p.name, statusIcon(status))
		b.WriteString("║ " + label + "\n")

		// Show agents under running phases.
		if status == PhaseRunning && len(data.Agents) > 0 {
			agents := sortedKeys(data.Agents)
			for _, name := range agents {
				st := data.Agents[name]
				line := fmt.Sprintf("  %s: %s (%s)", name, p.key, st)
				b.WriteString("║" + line + "\n")
			}
		}
	}

	// Blocker section.
	if data.Blocker != "" {
		b.WriteString(sep + "\n")
		b.WriteString(boxLine("Blocker: "+data.Blocker) + "\n")
	}

	// Elapsed time.
	b.WriteString(sep + "\n")
	b.WriteString(boxLine("Elapsed: "+FormatElapsed(data.Elapsed)) + "\n")
	b.WriteString(bot + "\n")

	return b.String()
}

// boxLine renders text padded inside box-drawing side borders.
func boxLine(text string) string {
	return fmt.Sprintf("║  %-*s║", boxWidth-2, text)
}

// FormatElapsed formats a duration into a human-readable string (e.g., "2m30s").
// Zero-value components are omitted.
func FormatElapsed(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, "")
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
