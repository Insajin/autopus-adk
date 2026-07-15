package design

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVisualGateReportAcceptsPassedPlaywrightAssertionWithoutFabricatingPixelDiff(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged: []string{"src/components/Home.tsx"},
		Assertions: []VisualAssertion{{
			Name:         "home-default.png",
			TestID:       "test-home",
			Project:      "desktop",
			Status:       "PASS",
			BaselinePath: ".autopus/baselines/visual/home-default.png",
			ComparisonID: "desktop/test-home/home-default.png",
		}},
		RequiredProjects: []string{"desktop"},
		ExecutedProjects: []string{"desktop"},
		SnapshotProof: SnapshotComparisonProof{UpdateSnapshots: "none", Projects: []SnapshotComparisonProject{
			{Name: "desktop", ComparisonStatus: "enabled"},
		}},
		Viewport:      "desktop",
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})

	require.Len(t, report.Assertions, 1)
	assert.Equal(t, "PASS", report.Assertions[0].Status)
	assert.Equal(t, 0, report.DiffSummary.PairsCompared)
	assert.Equal(t, int64(0), report.DiffSummary.TotalPixels)
	assert.Equal(t, "PASS", visualCheckV2ByID(t, report, "screenshot_capture").Status)
	assert.Equal(t, "PASS", visualCheckV2ByID(t, report, "screenshot_baseline").Status)
	diffCheck := visualCheckV2ByID(t, report, "screenshot_diff_summary")
	assert.Equal(t, "PASS", diffCheck.Status)
	assert.Contains(t, diffCheck.Message, "Playwright")
	assert.Contains(t, diffCheck.Message, "configured tolerance")
}

func TestBuildVisualGateReportTreatsExpectedArtifactAsBaselineEvidence(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged: []string{"src/components/Home.tsx"},
		Artifacts: []VisualArtifactV2{{
			Name:         "home-default-expected.png",
			Kind:         "expected",
			Path:         "test-results/home-default-expected.png",
			ComparisonID: "desktop/test-home/home-default.png",
		}},
		Viewport:      "desktop",
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})

	assert.Equal(t, "PASS", visualCheckV2ByID(t, report, "screenshot_baseline").Status)
}

func TestBuildVisualGateReportWarnsWhenAllWasRequestedButOnlyDesktopExecuted(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged:        []string{"src/components/Home.tsx"},
		RequiredProjects: []string{"desktop", "mobile", "tablet"},
		ExecutedProjects: []string{"desktop"},
		DesignContext:    Context{Found: true, SourcePath: "DESIGN.md"},
	})

	check := visualCheckV2ByID(t, report, "project_visual_coverage")
	assert.Equal(t, "WARN", check.Status)
	assert.Equal(t, []string{"desktop", "mobile", "tablet"}, check.Evidence)
}

func TestBuildVisualGateReportPassesWhenDesktopMobileTabletActuallyExecuted(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged:        []string{"src/components/Home.tsx"},
		RequiredProjects: []string{"tablet", "desktop", "mobile"},
		ExecutedProjects: []string{"tablet", "desktop", "mobile"},
		Assertions: []VisualAssertion{
			{Name: "desktop.png", Project: "desktop", Status: "PASS"},
			{Name: "mobile.png", Project: "mobile", Status: "PASS"},
			{Name: "tablet.png", Project: "tablet", Status: "PASS"},
		},
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})

	check := visualCheckV2ByID(t, report, "project_visual_coverage")
	assert.Equal(t, "PASS", check.Status)
	assert.Equal(t, []string{"desktop", "mobile", "tablet"}, check.Evidence)
	assert.Equal(t, []string{"desktop", "mobile", "tablet"}, report.ExecutedProjects)
}

func TestBuildVisualGateReportRedactsPrivateAssertionBaselinePath(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		UIChanged: []string{"src/components/Home.tsx"},
		Assertions: []VisualAssertion{{
			Name:         "home-default.png",
			TestID:       "test-home",
			Project:      "desktop",
			Status:       "PASS",
			BaselinePath: "/Users/alice/private/project/home-default.png",
			ComparisonID: "desktop/test-home/home-default.png",
		}},
		Viewport:         "desktop",
		ExecutedProjects: []string{"desktop"},
		DesignContext:    Context{Found: true, SourcePath: "DESIGN.md"},
	})

	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "/Users/alice/private")
}
