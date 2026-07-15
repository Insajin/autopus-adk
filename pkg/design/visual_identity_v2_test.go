package design

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVisualGateReportV2_StrictAnonymousSyntheticIdentityFails(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Strict: true,
		Assertions: []VisualAssertion{{
			Name:         "anonymous-screenshot-1.png",
			Anonymous:    true,
			Project:      "chromium",
			Status:       "PASS",
			BaselinePath: "snapshots/anonymous-screenshot-1.png",
		}},
	})

	check := visualCheckV2ByID(t, report, "screenshot_identity")
	assert.Equal(t, "FAIL", check.Status)
	assert.Equal(t, "FAIL", report.Verdict)
}

func TestBuildVisualGateReportV2_AdvisoryAnonymousSyntheticIdentityWarns(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Assertions: []VisualAssertion{{Name: "anonymous-screenshot-23.png", Anonymous: true, Status: "PASS"}},
	})

	assert.Equal(t, "WARN", visualCheckV2ByID(t, report, "screenshot_identity").Status)
}

func TestBuildVisualGateReportV2_ExplicitNamedScreenshotIdentityPasses(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Strict:     true,
		Assertions: []VisualAssertion{{Name: "home.png", Status: "PASS"}},
	})

	assert.Equal(t, "PASS", visualCheckV2ByID(t, report, "screenshot_identity").Status)
}

func TestBuildVisualGateReportV2_ExplicitSyntheticLookingNameIsNotAnonymous(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Strict:     true,
		Assertions: []VisualAssertion{{Name: "anonymous-screenshot-1.png", Status: "PASS"}},
	})

	assert.Equal(t, "PASS", visualCheckV2ByID(t, report, "screenshot_identity").Status)
}
