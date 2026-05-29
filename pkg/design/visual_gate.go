package design

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VisualGateInput struct {
	UIChanged      []string
	Screenshots    []string
	Artifacts      []VisualArtifact
	Viewport       string
	DesignContext  Context
	VisualCritic   VisualCriticReport
	MaxFixAttempts int
	PlaywrightErr  string
}

type VisualGateReport struct {
	Version        int                 `json:"version"`
	GeneratedAt    string              `json:"generated_at"`
	Verdict        string              `json:"verdict"`
	Viewport       string              `json:"viewport"`
	UIChanged      []string            `json:"ui_changed"`
	Screenshots    []string            `json:"screenshots"`
	Artifacts      []VisualArtifact    `json:"artifacts,omitempty"`
	DiffSummary    ScreenshotDiffStats `json:"screenshot_diff_summary"`
	VisualCritic   VisualCriticReport  `json:"visual_critic,omitempty"`
	MaxFixAttempts int                 `json:"max_fix_attempts"`
	Checks         []VisualCheck       `json:"checks"`
	PlaywrightErr  string              `json:"playwright_error,omitempty"`
}

type VisualCheck struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Evidence []string `json:"evidence,omitempty"`
}

type VisualArtifact struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ContentType string `json:"content_type,omitempty"`
	Path        string `json:"path"`
	LocalPath   string `json:"-"`
}

type ScreenshotDiffStats struct {
	PairsCompared     int      `json:"pairs_compared"`
	ChangedPixels     int64    `json:"changed_pixels"`
	TotalPixels       int64    `json:"total_pixels"`
	MaxChangedRatio   float64  `json:"max_changed_ratio"`
	DiffArtifactRefs  []string `json:"diff_artifact_refs,omitempty"`
	ComparisonErrors  []string `json:"comparison_errors,omitempty"`
	DeterministicMode string   `json:"deterministic_mode"`
}

func BuildVisualGateReport(input VisualGateInput) VisualGateReport {
	artifacts := sanitizeArtifacts(input.Artifacts)
	diff := BuildScreenshotDiffStats(artifacts)
	report := VisualGateReport{
		Version:        1,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Viewport:       input.Viewport,
		UIChanged:      sanitizePathList(input.UIChanged),
		Screenshots:    sanitizePathList(input.Screenshots),
		Artifacts:      publicArtifacts(artifacts),
		DiffSummary:    diff,
		VisualCritic:   sanitizeVisualCritic(input.VisualCritic),
		MaxFixAttempts: input.MaxFixAttempts,
		PlaywrightErr:  input.PlaywrightErr,
	}
	report.Checks = append(report.Checks, designContextCheck(input.DesignContext))
	report.Checks = append(report.Checks, screenshotCheck(report.Screenshots))
	report.Checks = append(report.Checks, viewportCheck(input.Viewport))
	report.Checks = append(report.Checks, baselineCheck(report.Screenshots))
	report.Checks = append(report.Checks, screenshotDiffCheck(diff))
	report.Checks = append(report.Checks, visualCriticCheck(report.VisualCritic))
	report.Checks = append(report.Checks, qameshHandoffCheck())
	report.Verdict = visualVerdict(report.Checks)
	return report
}

func WriteVisualGateReport(root string, report VisualGateReport) (string, error) {
	dir := filepath.Join(root, ".autopus", "design", "verify")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "latest.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	return path, os.WriteFile(path, data, 0o644)
}

func (r VisualGateReport) Summary(path string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "visual gate: %s", r.Verdict)
	if path != "" {
		fmt.Fprintf(&sb, " (%s)", filepath.ToSlash(path))
	}
	sb.WriteString("\n")
	for _, check := range r.Checks {
		fmt.Fprintf(&sb, "  - %s: %s — %s\n", check.ID, check.Status, check.Message)
	}
	return sb.String()
}

func designContextCheck(ctx Context) VisualCheck {
	if ctx.Found {
		return VisualCheck{ID: "design_context", Status: "PASS", Severity: "info", Message: "design context available", Evidence: []string{ctx.SourcePath}}
	}
	return VisualCheck{ID: "design_context", Status: "WARN", Severity: "medium", Message: "no design context configured; using inferred visual checks"}
}

func screenshotCheck(paths []string) VisualCheck {
	if len(paths) > 0 {
		return VisualCheck{ID: "screenshot_capture", Status: "PASS", Severity: "info", Message: "screenshots captured", Evidence: paths}
	}
	return VisualCheck{ID: "screenshot_capture", Status: "FAIL", Severity: "high", Message: "no screenshots were captured for UI changes"}
}

func viewportCheck(viewport string) VisualCheck {
	if viewport == "all" || strings.Contains(viewport, ",") {
		return VisualCheck{ID: "viewport_coverage", Status: "PASS", Severity: "info", Message: "multi-viewport coverage requested"}
	}
	if viewport == "" {
		viewport = "desktop"
	}
	return VisualCheck{ID: "viewport_coverage", Status: "WARN", Severity: "medium", Message: "single viewport only; run desktop,mobile,tablet or all for stronger coverage", Evidence: []string{viewport}}
}

func baselineCheck(paths []string) VisualCheck {
	for _, path := range paths {
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(lower, "snapshot") || strings.Contains(lower, "golden") {
			return VisualCheck{ID: "screenshot_baseline", Status: "PASS", Severity: "info", Message: "screenshot baseline or golden ref detected", Evidence: []string{path}}
		}
	}
	return VisualCheck{ID: "screenshot_baseline", Status: "WARN", Severity: "medium", Message: "no screenshot baseline/golden ref detected; visual comparison is weaker"}
}

func qameshHandoffCheck() VisualCheck {
	return VisualCheck{ID: "qamesh_handoff", Status: "PASS", Severity: "info", Message: "visual gate report is metadata-only and can be consumed by QAMESH"}
}

func screenshotDiffCheck(stats ScreenshotDiffStats) VisualCheck {
	if stats.PairsCompared > 0 {
		status := "PASS"
		if stats.MaxChangedRatio > 0 {
			status = "WARN"
		}
		return VisualCheck{
			ID:       "screenshot_diff_summary",
			Status:   status,
			Severity: "medium",
			Message:  fmt.Sprintf("compared %d screenshot pair(s), max changed ratio %.6f", stats.PairsCompared, stats.MaxChangedRatio),
		}
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
	return VisualCheck{
		ID:       "visual_critic",
		Status:   status,
		Severity: "medium",
		Message:  fmt.Sprintf("visual critic status %s with %d finding(s)", status, len(critic.Findings)),
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

func sanitizePathList(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		out = append(out, path)
	}
	return out
}

func sanitizeArtifacts(artifacts []VisualArtifact) []VisualArtifact {
	out := make([]VisualArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		artifact.Path = filepath.ToSlash(strings.TrimSpace(artifact.Path))
		artifact.LocalPath = strings.TrimSpace(artifact.LocalPath)
		if artifact.Path == "" {
			continue
		}
		if artifact.Kind == "" {
			artifact.Kind = ClassifyVisualArtifact(artifact.Name, artifact.Path)
		}
		out = append(out, artifact)
	}
	return out
}

func publicArtifacts(artifacts []VisualArtifact) []VisualArtifact {
	out := make([]VisualArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		artifact.LocalPath = ""
		out = append(out, artifact)
	}
	return out
}

func sanitizeVisualCritic(critic VisualCriticReport) VisualCriticReport {
	if critic.Status == "" {
		return VisualCriticReport{}
	}
	critic.Status = strings.ToUpper(strings.TrimSpace(critic.Status))
	for i := range critic.Findings {
		critic.Findings[i].Screenshot = filepath.ToSlash(strings.TrimSpace(critic.Findings[i].Screenshot))
	}
	return critic
}

func RedactVisualPath(root, rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(path)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "external:" + shortHash(path)
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	if rel, err := filepath.Rel(rootAbs, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		base = "artifact"
	}
	return filepath.ToSlash("external:" + shortHash(path) + ":" + base)
}
