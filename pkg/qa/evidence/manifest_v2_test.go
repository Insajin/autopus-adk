package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidate_AcceptsV2GenericChecksForProjectSurfaces(t *testing.T) {
	t.Parallel()

	for _, surface := range []string{"cli", "backend", "frontend", "desktop", "package", "custom", "multi"} {
		surface := surface
		t.Run(surface, func(t *testing.T) {
			t.Parallel()

			manifest := fixtureV2Manifest(t, surface)
			manifest.OracleResults.A11y = nil
			manifest.OracleResults.Desktop = nil

			require.NoError(t, manifest.Validate())
		})
	}
}

func TestManifestValidate_RejectsV2WithoutGenericChecksOrTraceFields(t *testing.T) {
	t.Parallel()

	missingChecks := fixtureV2Manifest(t, "package")
	missingChecks.OracleResults.Checks = nil
	require.ErrorContains(t, missingChecks.Validate(), "oracle_results.checks")

	missingJourney := fixtureV2Manifest(t, "package")
	missingJourney.SourceRefs.JourneyID = ""
	require.ErrorContains(t, missingJourney.Validate(), "source_refs.journey_id")

	missingStep := fixtureV2Manifest(t, "package")
	missingStep.SourceRefs.StepID = ""
	require.ErrorContains(t, missingStep.Validate(), "source_refs.step_id")

	missingAdapter := fixtureV2Manifest(t, "package")
	missingAdapter.SourceRefs.Adapter = ""
	require.ErrorContains(t, missingAdapter.Validate(), "source_refs.adapter")

	browser := fixtureV2Manifest(t, "browser")
	require.ErrorContains(t, browser.Validate(), "unsupported surface")
}

func TestNormalizeManifest_GenericFailedCheckSetsFailedStatus(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")
	manifest.Status = "passed"

	normalized := NormalizeManifest(manifest)

	assert.Equal(t, "failed", normalized.Status)
}

func TestWriteFeedbackBundle_IncludesV2FailedCheckAndJourneyContext(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")

	result, err := WriteFeedbackBundle(manifest, "codex", t.TempDir())

	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(result.BundlePath, "repair-prompt.md"))
	require.NoError(t, err)
	prompt := string(body)
	assert.Contains(t, prompt, "npm-test")
	assert.Contains(t, prompt, "unit_test")
	assert.Contains(t, prompt, "exit_code=0")
	assert.Contains(t, prompt, "exit_code=1")
	assert.Contains(t, prompt, "npm test exited with code 1")
	assert.Contains(t, prompt, "journey-node")
	assert.Contains(t, prompt, "step-test")
	assert.Contains(t, prompt, "node-script")
}

func fixtureV2Manifest(t *testing.T, surface string) Manifest {
	t.Helper()

	dir := t.TempDir()
	stdout := filepath.Join(dir, "stdout.log")
	require.NoError(t, os.WriteFile(stdout, []byte("npm test failed\n"), 0o644))

	return Manifest{
		SchemaVersion:       SchemaVersionV2,
		QAResultID:          "qa-node-package-001",
		Surface:             surface,
		Lane:                "fast",
		ScenarioRef:         "package:unit",
		Runner:              Runner{Name: "node-script", Command: "npm run test:qamesh"},
		Status:              "failed",
		StartedAt:           "2026-05-03T00:00:00Z",
		EndedAt:             "2026-05-03T00:00:01Z",
		DurationMS:          1000,
		RetentionClass:      "local-redacted",
		ReproductionCommand: "npm run test:qamesh",
		Artifacts: []ArtifactRef{{
			Kind:        "stdout",
			Path:        stdout,
			Publishable: true,
			Redaction:   "text_redacted_and_scanned",
		}},
		OracleResults: OracleResults{Checks: []CheckResult{{
			ID:             "npm-test",
			Type:           "unit_test",
			Status:         "failed",
			Expected:       "exit_code=0",
			Actual:         "exit_code=1",
			ArtifactRefs:   []string{"stdout"},
			FailureSummary: "npm test exited with code 1",
		}}},
		RedactionStatus: RedactionStatus{Status: "passed"},
		SourceRefs: SourceRefs{
			SourceSpec:       "SPEC-QAMESH-002",
			AcceptanceRefs:   []string{"AC-QAMESH2-006", "AC-QAMESH2-008"},
			OwnedPaths:       []string{"package.json", "src/**"},
			DoNotModifyPaths: []string{".codex/**"},
			JourneyID:        "journey-node",
			StepID:           "step-test",
			Adapter:          "node-script",
			OracleThresholds: map[string]any{"exit_code": 0},
		},
	}
}
