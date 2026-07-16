package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The JSONL fixture is a verbatim Playwright 1.58.2 blob report captured with ignoreSnapshots=true.
func TestActualPlaywright158IgnoredSnapshotsBlobRemainsUnproven(t *testing.T) {
	t.Parallel()

	report, err := os.ReadFile(filepath.Join("testdata", "playwright-1.58.2-ignore-snapshots.jsonl"))
	require.NoError(t, err)
	assert.NotContains(t, string(report), "ignoreSnapshots")
	evidence := collectVisualEvidence(buildBlobReportBytes(t, report, nil))

	require.Len(t, evidence.Assertions, 1)
	assert.Equal(t, "FAIL", evidence.Assertions[0].Status)
	assert.Contains(t, evidence.Assertions[0].Diagnostic, "not proven")

	disabledBlob, err := appendSnapshotProofToBlob(buildBlobReportBytes(t, report, nil), snapshotComparisonProof{
		Version:  1,
		Projects: []snapshotProjectProof{{Name: "chromium", ComparisonsEnabled: false}},
	})
	require.NoError(t, err)
	disabledEvidence := collectVisualEvidence(disabledBlob)
	require.Len(t, disabledEvidence.Assertions, 1)
	assert.Equal(t, "FAIL", disabledEvidence.Assertions[0].Status)
	assert.Contains(t, disabledEvidence.Assertions[0].Diagnostic, "unproven")

	err = writeVerifyVisualGate(
		t.TempDir(),
		[]string{"src/Home.tsx"},
		nil,
		nil,
		disabledEvidence.Assertions,
		disabledEvidence.Projects,
		"desktop",
		design.Context{Found: true, SourcePath: "DESIGN.md"},
		1,
		nil,
		true,
		"",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict visual gate failed")
}

func TestActualPlaywright158LegacyEnabledProofRemainsUnproven(t *testing.T) {
	t.Parallel()

	report, err := os.ReadFile(filepath.Join("testdata", "playwright-1.58.2-normal-snapshots.jsonl"))
	require.NoError(t, err)
	blob := buildBlobReportBytes(t, report, nil)
	blob, err = appendSnapshotProofToBlob(blob, snapshotComparisonProof{
		Version:  1,
		Projects: []snapshotProjectProof{{Name: "chromium", ComparisonsEnabled: true}},
	})
	require.NoError(t, err)

	evidence := collectVisualEvidence(blob)
	require.Len(t, evidence.Assertions, 1)
	assert.Equal(t, "FAIL", evidence.Assertions[0].Status)
	assert.Contains(t, evidence.Assertions[0].Diagnostic, "unproven")
}

func TestRunPlaywrightActual158SnapshotBoundary(t *testing.T) {
	fixtureDir := os.Getenv("AUTOPUS_PLAYWRIGHT_158_FIXTURE_DIR")
	if fixtureDir == "" {
		t.Skip("set AUTOPUS_PLAYWRIGHT_158_FIXTURE_DIR to run the Playwright 1.58.2 integration fixture")
	}
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(fixtureDir))

	t.Setenv("IGNORE_SNAPSHOTS", "")
	normalOutput, err := runPlaywright("desktop")
	require.NoError(t, err)
	normalEvidence := collectVisualEvidence(normalOutput)
	require.Len(t, normalEvidence.Assertions, 1)
	assert.Equal(t, "FAIL", normalEvidence.Assertions[0].Status)
	assert.Equal(t, "unproven", normalEvidence.SnapshotProofStatus)

	t.Setenv("IGNORE_SNAPSHOTS", "1")
	ignoredOutput, playwrightErr := runPlaywright("desktop")
	require.NoError(t, playwrightErr)
	ignoredEvidence := collectVisualEvidence(ignoredOutput)
	require.Len(t, ignoredEvidence.Assertions, 1)
	assert.Equal(t, "FAIL", ignoredEvidence.Assertions[0].Status)
	assert.Equal(t, "unproven", ignoredEvidence.SnapshotProofStatus)

	err = writeVerifyVisualGate(
		t.TempDir(), []string{"src/Home.tsx"}, nil, nil,
		ignoredEvidence.Assertions, ignoredEvidence.Projects, "desktop",
		design.Context{Found: true, SourcePath: "DESIGN.md"}, 1, playwrightErr, true, "",
	)
	require.Error(t, err)
}

func TestRunPlaywrightActual159PublicSnapshotBoundary(t *testing.T) {
	fixtureDir := os.Getenv("AUTOPUS_PLAYWRIGHT_159_FIXTURE_DIR")
	if fixtureDir == "" {
		t.Skip("set AUTOPUS_PLAYWRIGHT_159_FIXTURE_DIR to run the Playwright 1.59.1 integration fixture")
	}
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(fixtureDir))

	const project = "Visual Chrome Custom"
	t.Setenv("IGNORE_SNAPSHOTS", "")
	enabledOutput, err := runPlaywright(project)
	require.NoError(t, err)
	enabled := collectVisualEvidence(enabledOutput)
	assert.Equal(t, "enabled", enabled.SnapshotProofStatus)
	assert.Equal(t, []string{project}, enabled.RequiredProjects)
	require.Len(t, enabled.Assertions, 1)
	assert.Equal(t, "PASS", enabled.Assertions[0].Status)
	assert.Equal(t, project, enabled.Assertions[0].Project)

	t.Setenv("IGNORE_SNAPSHOTS", "1")
	disabledOutput, err := runPlaywright(project)
	require.NoError(t, err)
	disabled := collectVisualEvidence(disabledOutput)
	assert.Equal(t, "disabled", disabled.SnapshotProofStatus)
	assert.Equal(t, []string{project}, disabled.RequiredProjects)
	require.Len(t, disabled.Assertions, 1)
	assert.Equal(t, "FAIL", disabled.Assertions[0].Status)
}
