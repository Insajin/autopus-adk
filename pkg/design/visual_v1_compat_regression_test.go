package design

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVisualGateReport_V05070CheckOrderAndPlaywrightMeaning(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReport(VisualGateInput{
		Screenshots:   []string{"snapshots/home.png"},
		Viewport:      "all",
		DesignContext: Context{Found: true},
		VisualCritic:  VisualCriticReport{Status: "PASS"},
		PlaywrightErr: "exit status 1",
	})

	ids := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		ids = append(ids, check.ID)
	}
	assert.Equal(t, []string{
		"design_context",
		"screenshot_capture",
		"viewport_coverage",
		"screenshot_baseline",
		"screenshot_diff_summary",
		"visual_critic",
		"qamesh_handoff",
	}, ids)
	assert.Equal(t, "WARN", report.Verdict)
	assert.Equal(t, "exit status 1", report.PlaywrightErr)
}

func TestBuildVisualGateReport_V05070QAMESHHandoffContract(t *testing.T) {
	t.Parallel()

	check := qameshHandoffCheckV1()
	assert.Equal(t, VisualCheck{
		ID:       "qamesh_handoff",
		Status:   "PASS",
		Severity: "info",
		Message:  "visual gate report is metadata-only and can be consumed by QAMESH",
	}, check)
}

func TestViewportCheckV1_V05070MessageContract(t *testing.T) {
	t.Parallel()

	check := viewportCheckV1("desktop")
	assert.Equal(t, "single viewport only; run desktop,mobile,tablet or all for stronger coverage", check.Message)
}

func TestScreenshotDiffCheckV1_V05070ComparisonErrorsRemainMetadata(t *testing.T) {
	t.Parallel()

	check := screenshotDiffCheckV1(ScreenshotDiffStats{ComparisonErrors: []string{"decode failed"}})
	assert.Equal(t, "WARN", check.Status)
	assert.Equal(t, "no deterministic screenshot diff summary available", check.Message)
}
