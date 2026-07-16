package design

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVisualGateReportV2FailsWhenPlaywrightExecutionFails(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Strict:        true,
		UIChanged:     []string{"src/Home.tsx"},
		Screenshots:   []string{"snapshots/home.png"},
		Viewport:      "all",
		DesignContext: Context{Found: true},
		PlaywrightErr: "exit status 1",
	})

	assert.Equal(t, "FAIL", report.Verdict)
	check := visualCheckV2ByID(t, report, "playwright_execution")
	assert.Equal(t, "FAIL", check.Status)
	assert.Contains(t, check.Message, "exit status 1")
}

func TestBuildVisualGateReportV2FailsWhenDeterministicComparisonErrors(t *testing.T) {
	t.Parallel()

	privateDir := filepath.Join(t.TempDir(), "missing")
	report := BuildVisualGateReportV2(VisualGateInputV2{
		Strict:    true,
		UIChanged: []string{"src/Home.tsx"}, Screenshots: []string{"actual.png"},
		Artifacts: []VisualArtifactV2{
			{Name: "expected", Kind: "expected", Path: "expected.png", LocalPath: filepath.Join(privateDir, "expected.png")},
			{Name: "actual", Kind: "actual", Path: "actual.png", LocalPath: filepath.Join(privateDir, "actual.png")},
		},
		Viewport: "desktop", DesignContext: Context{Found: true},
	})

	assert.Equal(t, "FAIL", report.Verdict)
	assert.Equal(t, "FAIL", visualCheckV2ByID(t, report, "screenshot_diff_summary").Status)
}

func TestBuildVisualGateReportExplainsNamedScreenshotContract(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged: []string{"src/Home.tsx"}, Viewport: "desktop", DesignContext: Context{Found: true},
	})
	check := visualCheckV2ByID(t, report, "screenshot_capture")
	assert.Contains(t, check.Message, "explicit")
	assert.Contains(t, check.Message, "custom expect message")
}

func TestBuildVisualGateReportPreservesSafeNestedAssertionName(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Assertions: []VisualAssertion{{Name: "sections/home/default.png", Status: "PASS"}},
	})
	assert.Equal(t, "sections/home/default.png", report.Assertions[0].Name)
}

func visualCheckV2ByID(t *testing.T, report VisualGateReportV2, id string) VisualCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("visual v2 check %q not found", id)
	return VisualCheck{}
}
