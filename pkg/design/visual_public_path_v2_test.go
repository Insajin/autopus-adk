package design

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootlessVisualAbsolute_RecognizesPortableAbsoluteForms(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"/Users/alice/private.png",
		"//server/share/private.png",
		"C:/Users/alice/private.png",
	} {
		assert.True(t, rootlessVisualAbsolute(value), value)
	}
}

func TestBuildVisualGateReportV2_ProcessCWDAbsolutePath_IsAlwaysExternal(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)
	absolute := filepath.Join(cwd, "sibling-workspace", "private.png")
	report := BuildVisualGateReportV2(VisualGateInputV2{Screenshots: []string{absolute}})

	require.Len(t, report.Screenshots, 1)
	assert.True(t, strings.HasPrefix(report.Screenshots[0], "external:"), report.Screenshots[0])
}

func TestBuildVisualGateReportV2_ProcessCWDAssertionBaseline_IsAlwaysExternal(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)
	absolute := filepath.Join(cwd, "private", "baseline.png")
	report := BuildVisualGateReportV2(VisualGateInputV2{
		Assertions: []VisualAssertion{{Name: "home.png", Status: "PASS", BaselinePath: absolute}},
	})

	require.Len(t, report.Assertions, 1)
	assert.True(t, strings.HasPrefix(report.Assertions[0].BaselinePath, "external:"))
	assert.NotContains(t, report.Assertions[0].BaselinePath, "private")
}

func TestBuildVisualGateReportV2_AllPublicPathFields_AreSanitized(t *testing.T) {
	t.Parallel()

	report := BuildVisualGateReportV2(VisualGateInputV2{
		Artifacts: []VisualArtifactV2{{
			Name: "/Users/alice/private/actual.png", Kind: "actual", Path: "screenshots/actual.png",
		}},
		VisualCritic: VisualCriticReport{
			Status: "WARN", Source: "/Users/alice/private/critic.json",
			Findings: []VisualCriticFinding{{
				Severity: "WARN", Screenshot: "../private/critic-shot.png",
			}},
		},
		DesignContext: Context{Found: true, SourcePath: "/Users/alice/private/DESIGN.md"},
	})

	require.Len(t, report.Artifacts, 1)
	assert.Equal(t, "actual.png", report.Artifacts[0].Name)
	assert.True(t, strings.HasPrefix(report.VisualCritic.Source, "external:"), report.VisualCritic.Source)
	require.Len(t, report.VisualCritic.Findings, 1)
	assert.True(t, strings.HasPrefix(report.VisualCritic.Findings[0].Screenshot, "external:"), report.VisualCritic.Findings[0].Screenshot)
	contextCheck := visualCheckV2ByID(t, report, "design_context")
	require.Len(t, contextCheck.Evidence, 1)
	assert.True(t, strings.HasPrefix(contextCheck.Evidence[0], "external:"), contextCheck.Evidence[0])
}
