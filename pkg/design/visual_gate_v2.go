package design

import (
	"strings"
	"time"
)

// VisualArtifactV2 binds visual files to a comparison attempt.
type VisualArtifactV2 struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ContentType  string `json:"content_type,omitempty"`
	Path         string `json:"path"`
	LocalPath    string `json:"-"`
	ComparisonID string `json:"comparison_id,omitempty"`
	ResultID     string `json:"result_id,omitempty"`
	Retry        int    `json:"retry"`
}

// VisualAssertion records a Playwright screenshot comparison result.
type VisualAssertion struct {
	Name         string `json:"name"`
	Anonymous    bool   `json:"anonymous,omitempty"`
	TestID       string `json:"test_id,omitempty"`
	Project      string `json:"project,omitempty"`
	Status       string `json:"status"`
	BaselinePath string `json:"baseline_path,omitempty"`
	ComparisonID string `json:"comparison_id,omitempty"`
	ResultID     string `json:"result_id,omitempty"`
	Retry        int    `json:"retry"`
	Diagnostic   string `json:"diagnostic,omitempty"`
}

type SnapshotComparisonProject struct {
	Name             string `json:"name"`
	ComparisonStatus string `json:"comparison_status"`
}

type SnapshotComparisonProof struct {
	Status            string                      `json:"status,omitempty"`
	Diagnostic        string                      `json:"diagnostic,omitempty"`
	PlaywrightVersion string                      `json:"playwright_version"`
	UpdateSnapshots   string                      `json:"update_snapshots"`
	Projects          []SnapshotComparisonProject `json:"projects"`
}

type VisualGateInputV2 struct {
	Strict           bool                    `json:"strict"`
	UIChanged        []string                `json:"ui_changed"`
	Screenshots      []string                `json:"screenshots"`
	Artifacts        []VisualArtifactV2      `json:"artifacts"`
	Assertions       []VisualAssertion       `json:"assertions"`
	RequiredProjects []string                `json:"required_projects"`
	ExecutedProjects []string                `json:"executed_projects"`
	Viewport         string                  `json:"viewport"`
	DesignContext    Context                 `json:"design_context"`
	VisualCritic     VisualCriticReport      `json:"visual_critic"`
	MaxFixAttempts   int                     `json:"max_fix_attempts"`
	PlaywrightErr    string                  `json:"playwright_error"`
	SnapshotProof    SnapshotComparisonProof `json:"snapshot_proof"`
}

type VisualGateReportV2 struct {
	Version          int                     `json:"version"`
	GeneratedAt      string                  `json:"generated_at"`
	LegacySHA256     string                  `json:"legacy_sha256"`
	Strict           bool                    `json:"strict"`
	Verdict          string                  `json:"verdict"`
	Viewport         string                  `json:"viewport"`
	UIChanged        []string                `json:"ui_changed"`
	Screenshots      []string                `json:"screenshots"`
	Artifacts        []VisualArtifactV2      `json:"artifacts,omitempty"`
	Assertions       []VisualAssertion       `json:"assertions,omitempty"`
	RequiredProjects []string                `json:"required_projects"`
	ExecutedProjects []string                `json:"executed_projects"`
	SnapshotProof    SnapshotComparisonProof `json:"snapshot_proof"`
	DiffSummary      ScreenshotDiffStats     `json:"screenshot_diff_summary"`
	VisualCritic     VisualCriticReport      `json:"visual_critic,omitempty"`
	MaxFixAttempts   int                     `json:"max_fix_attempts"`
	Checks           []VisualCheck           `json:"checks"`
	PlaywrightErr    string                  `json:"playwright_error,omitempty"`
}

func BuildVisualGateReportV2(input VisualGateInputV2) VisualGateReportV2 {
	artifacts := sanitizeArtifactsV2(input.Artifacts)
	assertions := sanitizeAssertions(input.Assertions)
	sanitizePublicReferencesV2(artifacts, assertions)
	report := VisualGateReportV2{
		Version:          2,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		Strict:           input.Strict,
		Viewport:         input.Viewport,
		UIChanged:        sanitizePublicPathListV2(input.UIChanged),
		Screenshots:      sanitizePublicPathListV2(input.Screenshots),
		Artifacts:        publicArtifactsV2(artifacts),
		Assertions:       assertions,
		RequiredProjects: sortedUnique(input.RequiredProjects),
		ExecutedProjects: sortedUnique(input.ExecutedProjects),
		SnapshotProof:    sanitizeSnapshotProof(input.SnapshotProof),
		DiffSummary:      BuildScreenshotDiffStatsV2(artifacts),
		VisualCritic:     sanitizeVisualCriticV2(input.VisualCritic),
		MaxFixAttempts:   input.MaxFixAttempts,
		PlaywrightErr:    input.PlaywrightErr,
	}
	report.Checks = append(report.Checks, designContextCheckV2(input.DesignContext))
	report.Checks = append(report.Checks, screenshotCheckV2(report.Screenshots, assertions))
	report.Checks = append(report.Checks, screenshotIdentityCheckV2(input.Strict, assertions))
	report.Checks = append(report.Checks, snapshotComparisonPolicyCheck(input.Strict, report.RequiredProjects, report.SnapshotProof))
	report.Checks = append(report.Checks, projectVisualCoverageCheck(input.Strict, report.RequiredProjects, report.ExecutedProjects, assertions))
	report.Checks = append(report.Checks, baselineCheckV2(report.Screenshots, artifacts, assertions))
	report.Checks = append(report.Checks, screenshotDiffCheckV2(report.DiffSummary, assertions))
	report.Checks = append(report.Checks, visualCriticCheck(report.VisualCritic))
	report.Checks = append(report.Checks, playwrightExecutionCheck(report.PlaywrightErr))
	report.Checks = append(report.Checks, qameshHandoffCheckV2())
	report.Verdict = visualVerdict(report.Checks)
	return report
}

func sanitizePublicReferencesV2(artifacts []VisualArtifactV2, assertions []VisualAssertion) {
	for i := range artifacts {
		artifacts[i].Path = sanitizePublicVisualReference(artifacts[i].Path)
		artifacts[i].ComparisonID = sanitizePublicVisualReference(artifacts[i].ComparisonID)
	}
	for i := range assertions {
		assertions[i].BaselinePath = sanitizePublicVisualReference(assertions[i].BaselinePath)
		assertions[i].ComparisonID = sanitizePublicVisualReference(assertions[i].ComparisonID)
	}
}

func sanitizePublicPathListV2(values []string) []string {
	values = sanitizePathList(values)
	for i := range values {
		values[i] = sanitizePublicVisualReference(values[i])
	}
	return values
}

func sanitizePublicVisualReference(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}
	if rootlessVisualAbsolute(value) || hasParentTraversal(value) {
		return externalVisualPath(value)
	}
	return value
}

func rootlessVisualAbsolute(value string) bool {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	return strings.HasPrefix(value, "/") || hasVisualVolumePrefix(value)
}

func sanitizeVisualCriticV2(critic VisualCriticReport) VisualCriticReport {
	critic = sanitizeVisualCritic(critic)
	critic.Source = sanitizePublicVisualReference(critic.Source)
	for i := range critic.Findings {
		critic.Findings[i].Screenshot = sanitizePublicVisualReference(critic.Findings[i].Screenshot)
	}
	return critic
}

func designContextCheckV2(ctx Context) VisualCheck {
	check := designContextCheck(ctx)
	for i := range check.Evidence {
		check.Evidence[i] = sanitizePublicVisualReference(check.Evidence[i])
	}
	return check
}
