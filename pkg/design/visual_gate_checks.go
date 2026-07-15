package design

import (
	"fmt"
	"path/filepath"
	"strings"
)

func designContextCheck(ctx Context) VisualCheck {
	if ctx.Found {
		return VisualCheck{ID: "design_context", Status: "PASS", Severity: "info", Message: "design context available", Evidence: []string{ctx.SourcePath}}
	}
	return VisualCheck{ID: "design_context", Status: "WARN", Severity: "medium", Message: "no design context configured; using inferred visual checks"}
}

func screenshotCheckV1(paths []string) VisualCheck {
	if len(paths) > 0 {
		return VisualCheck{ID: "screenshot_capture", Status: "PASS", Severity: "info", Message: "screenshots captured", Evidence: paths}
	}
	return VisualCheck{ID: "screenshot_capture", Status: "FAIL", Severity: "high", Message: "no screenshots were captured for UI changes"}
}

func viewportCheckV1(viewport string) VisualCheck {
	if viewport == "all" || strings.Contains(viewport, ",") {
		return VisualCheck{ID: "viewport_coverage", Status: "PASS", Severity: "info", Message: "multi-viewport coverage requested"}
	}
	if viewport == "" {
		viewport = "desktop"
	}
	return VisualCheck{ID: "viewport_coverage", Status: "WARN", Severity: "medium", Message: "single viewport only; run desktop,mobile,tablet or all for stronger coverage", Evidence: []string{viewport}}
}

func baselineCheckV1(paths []string) VisualCheck {
	for _, candidate := range paths {
		lower := strings.ToLower(filepath.ToSlash(candidate))
		if strings.Contains(lower, "snapshot") || strings.Contains(lower, "golden") {
			return VisualCheck{ID: "screenshot_baseline", Status: "PASS", Severity: "info", Message: "screenshot baseline or golden ref detected", Evidence: []string{candidate}}
		}
	}
	return VisualCheck{ID: "screenshot_baseline", Status: "WARN", Severity: "medium", Message: "no screenshot baseline/golden ref detected; visual comparison is weaker"}
}

func screenshotDiffCheckV1(stats ScreenshotDiffStats) VisualCheck {
	if stats.PairsCompared > 0 {
		status := "PASS"
		if stats.MaxChangedRatio > 0 {
			status = "WARN"
		}
		return VisualCheck{ID: "screenshot_diff_summary", Status: status, Severity: "medium", Message: fmt.Sprintf("compared %d screenshot pair(s), max changed ratio %.6f", stats.PairsCompared, stats.MaxChangedRatio)}
	}
	if len(stats.DiffArtifactRefs) > 0 {
		return VisualCheck{ID: "screenshot_diff_summary", Status: "WARN", Severity: "medium", Message: "Playwright diff artifact refs detected without local actual/expected comparison", Evidence: stats.DiffArtifactRefs}
	}
	return VisualCheck{ID: "screenshot_diff_summary", Status: "WARN", Severity: "medium", Message: "no deterministic screenshot diff summary available"}
}

func visualCriticCheck(critic VisualCriticReport) VisualCheck {
	if critic.Status == "" {
		return VisualCheck{ID: "visual_critic", Status: "WARN", Severity: "medium", Message: "no VLM visual critic report attached"}
	}
	status := strings.ToUpper(critic.Status)
	if status != "PASS" && status != "WARN" && status != "FAIL" {
		status = "WARN"
	}
	return VisualCheck{ID: "visual_critic", Status: status, Severity: "medium", Message: fmt.Sprintf("visual critic status %s with %d finding(s)", status, len(critic.Findings))}
}

func qameshHandoffCheckV1() VisualCheck {
	return VisualCheck{
		ID:       "qamesh_handoff",
		Status:   "PASS",
		Severity: "info",
		Message:  "visual gate report is metadata-only and can be consumed by QAMESH",
	}
}

func qameshHandoffCheckV2() VisualCheck {
	return VisualCheck{
		ID:       "qamesh_handoff",
		Status:   "WARN",
		Severity: "medium",
		Message:  "QAMESH handoff candidate; ingestion unproven",
	}
}

func visualVerdict(checks []VisualCheck) string {
	verdict := "PASS"
	for _, check := range checks {
		switch check.Status {
		case "FAIL":
			return "FAIL"
		case "WARN":
			verdict = "WARN"
		}
	}
	return verdict
}
