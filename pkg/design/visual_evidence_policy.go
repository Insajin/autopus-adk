package design

import (
	"sort"
	"strings"
)

func snapshotComparisonPolicyCheck(strict bool, required []string, proof SnapshotComparisonProof) VisualCheck {
	valid := strings.TrimSpace(proof.Status) == "enabled" &&
		strings.TrimSpace(proof.PlaywrightVersion) != "" &&
		strings.TrimSpace(proof.UpdateSnapshots) == "none" && len(required) > 0
	statuses := make(map[string]string, len(proof.Projects))
	for _, project := range proof.Projects {
		name := strings.TrimSpace(project.Name)
		status := strings.ToLower(strings.TrimSpace(project.ComparisonStatus))
		if prior, exists := statuses[name]; exists && prior != status {
			status = "unknown"
		}
		statuses[name] = status
	}
	for _, name := range required {
		if statuses[name] != "enabled" {
			valid = false
		}
	}
	if valid {
		return VisualCheck{ID: "snapshot_comparison_policy", Status: "PASS", Severity: "info", Message: "snapshot comparison is enabled and updateSnapshots is none", Evidence: required}
	}
	return VisualCheck{ID: "snapshot_comparison_policy", Status: advisoryFailureStatus(strict), Severity: "high", Message: "snapshot comparison proof requires updateSnapshots=none and enabled comparison for every required project", Evidence: required}
}

func projectVisualCoverageCheck(strict bool, required, executed []string, assertions []VisualAssertion) VisualCheck {
	final := finalVisualAssertions(assertions)
	executedSet := stringSet(executed)
	valid := len(required) > 0
	for _, project := range required {
		passes, failures := 0, 0
		for _, assertion := range final {
			if assertion.Project != project {
				continue
			}
			switch assertion.Status {
			case "PASS":
				passes++
			case "FAIL":
				failures++
			}
		}
		if _, ok := executedSet[project]; !ok || passes == 0 || failures > 0 {
			valid = false
		}
	}
	if valid {
		return VisualCheck{ID: "project_visual_coverage", Status: "PASS", Severity: "info", Message: "every required project has a final passing visual assertion", Evidence: required}
	}
	return VisualCheck{ID: "project_visual_coverage", Status: advisoryFailureStatus(strict), Severity: "high", Message: "each required project must execute and have at least one final PASS assertion with no final FAIL", Evidence: required}
}

func finalVisualAssertions(assertions []VisualAssertion) []VisualAssertion {
	byIdentity := make(map[string]VisualAssertion, len(assertions))
	for _, assertion := range assertions {
		identity := assertion.ComparisonID
		if identity == "" {
			identity = strings.Join([]string{assertion.Project, assertion.TestID, assertion.Name}, "\x00")
		}
		key := assertion.Project + "\x00" + identity
		prior, exists := byIdentity[key]
		if !exists || assertion.Retry >= prior.Retry {
			byIdentity[key] = assertion
		}
	}
	out := make([]VisualAssertion, 0, len(byIdentity))
	for _, assertion := range byIdentity {
		out = append(out, assertion)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project == out[j].Project {
			return out[i].Name < out[j].Name
		}
		return out[i].Project < out[j].Project
	})
	return out
}

func screenshotCheckV2(paths []string, assertions []VisualAssertion) VisualCheck {
	if len(paths) > 0 || hasPassedAssertion(assertions) {
		return VisualCheck{ID: "screenshot_capture", Status: "PASS", Severity: "info", Message: "screenshots captured", Evidence: paths}
	}
	return VisualCheck{ID: "screenshot_capture", Status: "FAIL", Severity: "high", Message: "no screenshots were captured; use an explicit toHaveScreenshot('name.png') call without a custom expect message"}
}

func screenshotIdentityCheckV2(strict bool, assertions []VisualAssertion) VisualCheck {
	for _, assertion := range finalVisualAssertions(assertions) {
		if assertion.Anonymous {
			return VisualCheck{
				ID:       "screenshot_identity",
				Status:   advisoryFailureStatus(strict),
				Severity: "high",
				Message:  "anonymous screenshot identity is not stable evidence; use an explicit toHaveScreenshot('name.png') call",
				Evidence: []string{assertion.Name},
			}
		}
	}
	return VisualCheck{ID: "screenshot_identity", Status: "PASS", Severity: "info", Message: "screenshot assertions use stable explicit identities"}
}

func baselineCheckV2(paths []string, artifacts []VisualArtifactV2, assertions []VisualAssertion) VisualCheck {
	for _, artifact := range artifacts {
		if artifact.Kind == "expected" {
			return VisualCheck{ID: "screenshot_baseline", Status: "PASS", Severity: "info", Message: "expected screenshot artifact detected", Evidence: []string{artifact.Path}}
		}
	}
	for _, assertion := range assertions {
		if assertion.Status == "PASS" && assertion.BaselinePath != "" {
			return VisualCheck{ID: "screenshot_baseline", Status: "PASS", Severity: "info", Message: "Playwright screenshot baseline comparison passed", Evidence: []string{assertion.BaselinePath}}
		}
	}
	return baselineCheckV1(paths)
}

func screenshotDiffCheckV2(stats ScreenshotDiffStats, assertions []VisualAssertion) VisualCheck {
	if hasFailedAssertion(finalVisualAssertions(assertions)) {
		return VisualCheck{ID: "screenshot_diff_summary", Status: "FAIL", Severity: "high", Message: "Playwright screenshot assertion failed"}
	}
	if len(stats.ComparisonErrors) > 0 {
		return VisualCheck{ID: "screenshot_diff_summary", Status: "FAIL", Severity: "high", Message: "deterministic screenshot comparison failed", Evidence: stats.ComparisonErrors}
	}
	check := screenshotDiffCheckV1(stats)
	if stats.PairsCompared == 0 && len(stats.ComparisonErrors) == 0 && hasPassedAssertion(assertions) {
		check = VisualCheck{ID: "screenshot_diff_summary", Status: "PASS", Severity: "info", Message: "Playwright screenshot comparison passed within configured tolerance"}
	}
	return check
}

func advisoryFailureStatus(strict bool) string {
	if strict {
		return "FAIL"
	}
	return "WARN"
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
