package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// FormatLeadTime renders a body-free, markdown-style lead-time report. Only
// names, counts, and durations appear; targets and free text never do.
func FormatLeadTime(report LeadTimeReport, comparison *LeadTimeComparison) string {
	var b strings.Builder

	b.WriteString("## Lead Time\n\n")
	fmt.Fprintf(&b, "SPEC: %s\n", report.SpecID)
	fmt.Fprintf(&b, "Completion: %s\n", formatDuration(report.CompletionLeadTime))
	fmt.Fprintf(&b, "Time to first slice: %s\n", nullableDuration(report.TimeToFirstSlice))
	fmt.Fprintf(&b, "Critical path: %s (%s)\n", joinPath(report.CriticalPath), formatDuration(report.CriticalPathDuration))
	fmt.Fprintf(&b, "Rereads: %d%s\n", report.RereadCount, formatReasons(report.Rereads))
	fmt.Fprintf(&b, "Reruns: %d%s\n", report.RerunCount, formatReasons(report.Reruns))
	fmt.Fprintf(&b, "Defects by discovery phase: %s\n", formatDefects(report.DefectsByDiscoveryPhase))
	fmt.Fprintf(&b, "Repeat finding rate: %.1f%%\n", report.RepeatFindingRate*100)
	fmt.Fprintf(&b, "Escaped defects: %d\n", report.EscapedDefects)
	fmt.Fprintf(&b, "Unresolved safety gates: %s\n", formatSafetyGates(report))
	if estimate := report.EstimateVsActual; estimate != nil {
		verdict := "within range"
		if !estimate.WithinRange {
			verdict = "outside range"
		}
		fmt.Fprintf(&b, "Estimate: %s-%s, actual %s (%s)\n",
			formatDuration(estimate.Min), formatDuration(estimate.Max), formatDuration(estimate.Actual), verdict)
	} else {
		b.WriteString("Estimate: n/a\n")
	}
	for _, issue := range report.Issues {
		fmt.Fprintf(&b, "Issue: %s\n", issue)
	}

	if comparison != nil {
		fmt.Fprintf(&b, "\n### Baseline: %s\n", comparison.BaselineSpecID)
		fmt.Fprintf(&b, "Delta first slice: %s\n", nullableSignedDuration(comparison.DeltaFirstSlice))
		fmt.Fprintf(&b, "Delta completion: %s\n", signedDuration(comparison.DeltaCompletion))
		fmt.Fprintf(&b, "Delta critical path: %s\n", signedDuration(comparison.DeltaCriticalPath))
		fmt.Fprintf(&b, "Escaped defects: %d -> %d\n", comparison.BaselineEscapedDefects, report.EscapedDefects)
		fmt.Fprintf(&b, "Unresolved safety gates: %d -> %d\n", comparison.BaselineUnresolvedSafetyGates, len(report.UnresolvedSafetyGates))
		fmt.Fprintf(&b, "Regression: %t\n", comparison.Regression)
		for _, reason := range comparison.RegressionReasons {
			fmt.Fprintf(&b, "Regression reason: %s\n", reason)
		}
	}
	return b.String()
}

func joinPath(path []string) string {
	if len(path) == 0 {
		return "-"
	}
	return strings.Join(path, " -> ")
}

func formatReasons(byReason map[string]int) string {
	if len(byReason) == 0 {
		return ""
	}
	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		parts = append(parts, fmt.Sprintf("%s: %d", reason, byReason[reason]))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func formatDefects(byPhase map[string]PhaseDefects) string {
	if len(byPhase) == 0 {
		return "none"
	}
	phases := make([]string, 0, len(byPhase))
	for phase := range byPhase {
		phases = append(phases, phase)
	}
	sort.Strings(phases)
	parts := make([]string, 0, len(phases))
	for _, phase := range phases {
		bucket := byPhase[phase]
		parts = append(parts, fmt.Sprintf("%s: %d (%d files)", phase, bucket.Count, bucket.FilesTouched))
	}
	return strings.Join(parts, ", ")
}

func formatSafetyGates(report LeadTimeReport) string {
	if len(report.UnresolvedSafetyGates) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(report.UnresolvedSafetyGates))
	for _, gate := range report.UnresolvedSafetyGates {
		parts = append(parts, fmt.Sprintf("%s (%s)", gate, report.SafetyGateStatus[gate]))
	}
	return strings.Join(parts, ", ")
}

func nullableDuration(value *time.Duration) string {
	if value == nil {
		return "n/a"
	}
	return formatDuration(*value)
}

func nullableSignedDuration(value *time.Duration) string {
	if value == nil {
		return "n/a"
	}
	return signedDuration(*value)
}

func signedDuration(value time.Duration) string {
	switch rounded := value.Round(time.Second); {
	case rounded < 0:
		return "-" + formatDuration(-rounded)
	case rounded > 0:
		return "+" + formatDuration(rounded)
	default:
		return "0s"
	}
}
