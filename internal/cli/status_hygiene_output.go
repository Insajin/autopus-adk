package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

const maxHygieneTextPaths = 5
const maxHygieneCheckPaths = 10

func (r statusHygieneReport) payload() statusHygienePayload {
	return statusHygienePayload{
		Available:         r.Available,
		Status:            r.Status,
		GeneratedDrift:    r.metricPayload(r.GeneratedDrift, "generated/runtime working tree drift candidates"),
		TrackedButIgnored: r.metricPayload(r.TrackedButIgnored, "tracked files matched by .gitignore"),
		RuntimeUnignored:  r.metricPayload(r.RuntimeUnignored, "untracked runtime/generated paths visible to git"),
		Diagnostic:        r.Diagnostic,
	}
}

func (r statusHygieneReport) metricPayload(paths []string, label string) statusHygieneMetricPayload {
	if !r.Available {
		return statusHygieneMetricPayload{
			Status:  "unavailable",
			Message: "diagnostic unavailable",
		}
	}
	if len(paths) == 0 {
		return statusHygieneMetricPayload{
			Status:  "ok",
			Message: "none observed",
		}
	}
	return statusHygieneMetricPayload{
		Status:  "warn",
		Count:   len(paths),
		Paths:   append([]string{}, paths...),
		Message: fmt.Sprintf("%d %s", len(paths), label),
	}
}

func hygieneJSONWarnings(report statusHygieneReport) []jsonMessage {
	if !report.Available {
		return []jsonMessage{{
			Code:    "hygiene_unavailable",
			Message: report.Diagnostic,
		}}
	}

	var warnings []jsonMessage
	if len(report.GeneratedDrift) > 0 {
		warnings = append(warnings, jsonMessage{
			Code:    "hygiene_generated_drift",
			Message: fmt.Sprintf("%d generated/runtime working tree drift candidate(s).", len(report.GeneratedDrift)),
		})
	}
	if len(report.TrackedButIgnored) > 0 {
		warnings = append(warnings, jsonMessage{
			Code:    "hygiene_tracked_but_ignored",
			Message: fmt.Sprintf("%d tracked file(s) are matched by .gitignore.", len(report.TrackedButIgnored)),
		})
	}
	if len(report.RuntimeUnignored) > 0 {
		warnings = append(warnings, jsonMessage{
			Code:    "hygiene_runtime_unignored",
			Message: fmt.Sprintf("%d untracked runtime/generated path(s) are visible to git.", len(report.RuntimeUnignored)),
		})
	}
	return warnings
}

func hygieneJSONChecks(scope string, report statusHygieneReport) []jsonCheck {
	return []jsonCheck{
		hygieneMetricCheck(scope, "generated_drift", "generated/runtime working tree drift", report.Available, report.Diagnostic, report.GeneratedDrift),
		hygieneMetricCheck(scope, "tracked_but_ignored", "tracked-but-ignored files", report.Available, report.Diagnostic, report.TrackedButIgnored),
		hygieneMetricCheck(scope, "runtime_unignored", "runtime/generated unignored files", report.Available, report.Diagnostic, report.RuntimeUnignored),
	}
}

func hygieneMetricCheck(scope, key, label string, available bool, diagnostic string, paths []string) jsonCheck {
	check := jsonCheck{
		ID:       fmt.Sprintf("%s.hygiene.%s", scope, key),
		Severity: "info",
		Status:   "pass",
		Detail:   fmt.Sprintf("%s: none observed", label),
	}
	if !available {
		check.Severity = "warning"
		check.Status = "warn"
		check.Detail = fmt.Sprintf("%s: diagnostic unavailable (%s)", label, diagnostic)
		return check
	}
	if len(paths) > 0 {
		check.Severity = "warning"
		check.Status = "warn"
		check.Detail = fmt.Sprintf("%s: %d candidate(s): %s", label, len(paths), formatHygieneCheckPaths(paths))
	}
	return check
}

func applyHygieneJSON(
	scope string,
	report statusHygieneReport,
	warnings *[]jsonMessage,
	checks *[]jsonCheck,
	status *jsonEnvelopeStatus,
) {
	*warnings = append(*warnings, hygieneJSONWarnings(report)...)
	*checks = append(*checks, hygieneJSONChecks(scope, report)...)
	if report.hasWarning() && *status == jsonStatusOK {
		*status = jsonStatusWarn
	}
}

func renderHygieneText(out io.Writer, report statusHygieneReport) {
	tui.SectionHeader(out, "Hygiene")
	if !report.Available {
		tui.SKIP(out, fmt.Sprintf("diagnostic unavailable: %s", report.Diagnostic))
		return
	}

	renderHygieneMetric(out, "generated/runtime working tree drift", report.GeneratedDrift)
	renderHygieneMetric(out, "tracked-but-ignored files", report.TrackedButIgnored)
	renderHygieneMetric(out, "runtime/generated unignored files", report.RuntimeUnignored)
}

func renderHygieneMetric(out io.Writer, label string, paths []string) {
	if len(paths) == 0 {
		tui.OK(out, fmt.Sprintf("%s: none observed", label))
		return
	}
	tui.SKIP(out, fmt.Sprintf("%s: %d candidate(s)", label, len(paths)))
	for _, rel := range limitHygieneTextPaths(paths) {
		tui.Bullet(out, rel)
	}
	if len(paths) > maxHygieneTextPaths {
		tui.Bullet(out, fmt.Sprintf("... and %d more", len(paths)-maxHygieneTextPaths))
	}
}

func limitHygieneTextPaths(paths []string) []string {
	if len(paths) <= maxHygieneTextPaths {
		return paths
	}
	return paths[:maxHygieneTextPaths]
}

func formatHygieneCheckPaths(paths []string) string {
	if len(paths) <= maxHygieneCheckPaths {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf(
		"%s, ... and %d more",
		strings.Join(paths[:maxHygieneCheckPaths], ", "),
		len(paths)-maxHygieneCheckPaths,
	)
}
