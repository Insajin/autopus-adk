package design

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// VisualGateInput is the released v1 input contract.
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

// VisualGateReport is the released latest.json v1 contract.
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

// VisualArtifact is the released v1 artifact shape.
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
	report := VisualGateReport{
		Version:        1,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Viewport:       input.Viewport,
		UIChanged:      sanitizePathList(input.UIChanged),
		Screenshots:    sanitizePathList(input.Screenshots),
		Artifacts:      publicArtifacts(artifacts),
		DiffSummary:    BuildScreenshotDiffStats(artifacts),
		VisualCritic:   sanitizeVisualCritic(input.VisualCritic),
		MaxFixAttempts: input.MaxFixAttempts,
		PlaywrightErr:  input.PlaywrightErr,
	}
	report.Checks = append(report.Checks, designContextCheck(input.DesignContext))
	report.Checks = append(report.Checks, screenshotCheckV1(report.Screenshots))
	report.Checks = append(report.Checks, viewportCheckV1(input.Viewport))
	report.Checks = append(report.Checks, baselineCheckV1(report.Screenshots))
	report.Checks = append(report.Checks, screenshotDiffCheckV1(report.DiffSummary))
	report.Checks = append(report.Checks, visualCriticCheck(report.VisualCritic))
	report.Checks = append(report.Checks, qameshHandoffCheckV1())
	report.Verdict = visualVerdict(report.Checks)
	return report
}

func WriteVisualGateReport(root string, report VisualGateReport) (string, error) {
	return writeVisualGateReport(root, report)
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
