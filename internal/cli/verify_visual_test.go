package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVerifyVisualGate_StrictFailsAfterWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	err := writeVerifyVisualGate(root, []string{"src/components/Button.tsx"}, nil, nil, nil, nil, "desktop", design.Context{Found: true, SourcePath: "DESIGN.md"}, 2, nil, true, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict visual gate failed")
	assert.FileExists(t, filepath.Join(root, ".autopus", "design", "verify", "latest.json"))
}

func TestWriteVerifyVisualGateMergesVisualCriticReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	critic := filepath.Join(root, ".autopus", "design", "critic.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(critic), 0o755))
	require.NoError(t, os.WriteFile(critic, []byte(`{"status":"FAIL","findings":[{"severity":"FAIL","category":"overlap","message":"button overlaps"}]}`), 0o644))

	err := writeVerifyVisualGate(root, []string{"src/components/Button.tsx"}, []string{"shot.png"}, nil, nil, nil, "all", design.Context{Found: true, SourcePath: "DESIGN.md"}, 2, nil, true, ".autopus/design/critic.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict visual gate failed")
}

func TestCollectVisualArtifactsRedactsExternalAbsolutePaths(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{{
		Specs: []playwrightSpec{{Tests: []playwrightTest{{Results: []playwrightTestResult{{Attachments: []playwrightAttachment{
			{Name: "actual", Path: "/tmp/playwright/actual.png"},
			{Name: "expected", Path: "/tmp/playwright/expected.png"},
			{Name: "diff", Path: "/tmp/playwright/diff.png"},
		}}}}}}},
	}})

	artifacts := collectVisualArtifacts(input)
	require.Len(t, artifacts, 3)
	assert.Equal(t, "actual", artifacts[0].Kind)
	assert.Contains(t, artifacts[0].Path, "external:")
	assert.Equal(t, "/tmp/playwright/actual.png", artifacts[0].LocalPath)
}

func TestCollectVisualEvidenceRecursesNestedJSONSuitesForFailureAttachments(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"config":{"projects":[{"name":"desktop"}]},
		"suites":[{"title":"root","suites":[{"title":"nested","suites":[{
			"title":"leaf",
			"specs":[{"id":"spec-home","tests":[{"projectName":"desktop","results":[{
				"status":"failed",
				"attachments":[
					{"name":"home-default-expected.png","contentType":"image/png","path":"baselines/home-default.png"},
					{"name":"home-default-actual.png","contentType":"image/png","path":"test-results/run/home-default-actual.png"},
					{"name":"home-default-diff.png","contentType":"image/png","path":"test-results/run/home-default-diff.png"}
				]
			}]}]}]
		}]}]}]
	}`)

	evidence := collectVisualEvidence(input)
	require.Len(t, evidence.Artifacts, 3)
	assert.Equal(t, []string{"expected", "actual", "diff"}, []string{
		evidence.Artifacts[0].Kind,
		evidence.Artifacts[1].Kind,
		evidence.Artifacts[2].Kind,
	})
	assert.Equal(t, []string{"desktop"}, evidence.Projects)
	assert.NotEmpty(t, evidence.Artifacts[0].ComparisonID)
	assert.Equal(t, evidence.Artifacts[0].ComparisonID, evidence.Artifacts[1].ComparisonID)
	assert.Equal(t, evidence.Artifacts[1].ComparisonID, evidence.Artifacts[2].ComparisonID)
}

func TestCollectVisualEvidencePreservesLegacyJSONAttachmentFallback(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{{
		Specs: []playwrightSpec{{Tests: []playwrightTest{{Results: []playwrightTestResult{{Attachments: []playwrightAttachment{
			{Name: "screenshot", ContentType: "image/png", Path: "test-results/legacy.png"},
		}}}}}}},
	}})

	evidence := collectVisualEvidence(input)
	require.Len(t, evidence.Artifacts, 1)
	assert.Equal(t, "test-results/legacy.png", evidence.Artifacts[0].Path)
}

func TestWriteVerifyVisualGatePersistsAssertionsAndExecutedProjects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	assertions := []design.VisualAssertion{{
		Name:         "home-default.png",
		TestID:       "test-home",
		Project:      "desktop",
		Status:       "PASS",
		BaselinePath: ".autopus/baselines/visual/home-default.png",
		ComparisonID: "desktop/test-home/home-default.png",
	}}
	err := writeVerifyVisualGate(
		root,
		[]string{"src/components/Home.tsx"},
		nil,
		nil,
		assertions,
		[]string{"desktop", "mobile", "tablet"},
		"all",
		design.Context{Found: true, SourcePath: "DESIGN.md"},
		2,
		nil,
		false,
		"",
	)
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(root, ".autopus", "design", "verify", "latest.v2.json"))
	require.NoError(t, err)
	var report design.VisualGateReportV2
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.Equal(t, assertions, report.Assertions)
	assert.Equal(t, []string{"desktop", "mobile", "tablet"}, report.ExecutedProjects)
}
