package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidate_AcceptsBrowserFailure(t *testing.T) {
	t.Parallel()

	manifest := fixtureManifest(t, "browser", "failed")

	require.NoError(t, manifest.Validate())
}

func TestManifestValidate_AcceptsDesktopMappedBundle(t *testing.T) {
	t.Parallel()

	manifest := fixtureManifest(t, "desktop", "passed")
	manifest.OracleResults = OracleResults{
		Desktop: &DesktopOracle{TimeoutClassification: "driver_timeout"},
	}
	manifest.SourceRefs.SourceSpec = "SPEC-DESKTOP-017"

	require.NoError(t, manifest.Validate())
}

func TestManifestValidate_RejectsInvalidSurfaceAndMissingRequiredFields(t *testing.T) {
	t.Parallel()

	invalidSurface := fixtureManifest(t, "mobile", "failed")
	require.ErrorContains(t, invalidSurface.Validate(), "unsupported surface")

	missingID := fixtureManifest(t, "browser", "failed")
	missingID.QAResultID = ""
	require.ErrorContains(t, missingID.Validate(), "qa_result_id")
}

func TestResolveArtifactPaths_RejectsPathsOutsideInputRoot(t *testing.T) {
	t.Parallel()

	inputRoot := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.json")
	require.NoError(t, os.WriteFile(outside, []byte(`{"access_token":"sk-proj-qameshfake1234567890"}`), 0o644))
	manifest := fixtureManifest(t, "browser", "failed")
	manifest.Artifacts = []ArtifactRef{{
		Kind:        "console",
		Path:        outside,
		Publishable: true,
		Redaction:   "text_redacted_and_scanned",
	}}

	_, err := ResolveArtifactPaths(manifest, inputRoot)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "qameshfake")
}

func TestWriteFinalManifest_RejectsUnsafeArtifactBeforePublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raw := filepath.Join(dir, "trace.zip")
	require.NoError(t, os.WriteFile(raw, []byte("Authorization: Bearer sk-proj-qameshfake1234567890"), 0o644))
	manifest := fixtureManifest(t, "browser", "failed")
	manifest.Artifacts = []ArtifactRef{{
		Kind:        "trace_sanitized",
		Path:        raw,
		Publishable: true,
		Redaction:   "text_redacted_and_scanned",
	}}

	_, err := WriteFinalManifest(manifest, filepath.Join(dir, "final"))
	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(dir, "final"))
}

func TestWriteFinalManifest_CopiesNonPublishableQuarantineSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	summary := filepath.Join(dir, "screenshot-quarantined.json")
	require.NoError(t, os.WriteFile(summary, []byte(`{"publishable":false}`), 0o644))
	manifest := fixtureManifest(t, "browser", "failed")
	manifest.Artifacts = append(manifest.Artifacts, ArtifactRef{
		Kind:        "screenshot_quarantined",
		Path:        summary,
		Publishable: false,
		Redaction:   "local_only_quarantine_ref",
	})

	manifestPath, err := WriteFinalManifest(manifest, filepath.Join(dir, "final"))

	require.NoError(t, err)
	body, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "artifacts/screenshot_quarantined/screenshot-quarantined.json")
	assert.FileExists(t, filepath.Join(dir, "final", "artifacts", "screenshot_quarantined", "screenshot-quarantined.json"))
}

func TestNormalizeManifest_A11yCriticalFailureWinsOverRunnerStatus(t *testing.T) {
	t.Parallel()

	manifest := fixtureManifest(t, "browser", "passed")
	manifest.OracleResults.A11y.CriticalCount = 1

	normalized := NormalizeManifest(manifest)

	assert.Equal(t, "failed", normalized.Status)
}

func TestValidateLocatorConvention_RequiresFallbackReasonForCSS(t *testing.T) {
	t.Parallel()

	result := ValidateLocatorConvention([]LocatorContract{
		{Strategy: "role", Value: `button[name="Continue"]`},
		{Strategy: "label", Value: "Email"},
		{Strategy: "css", Value: "#app > div:nth-child(2) > button"},
	})

	assert.ElementsMatch(t, []string{"role", "label"}, result.Accepted)
	assert.Contains(t, result.Rejected, "css")
	assert.True(t, result.FallbackReasonRequired)
	assert.True(t, result.StableTestIDRequired)
}

func fixtureManifest(t *testing.T, surface, status string) Manifest {
	t.Helper()
	dir := t.TempDir()
	artifact := filepath.Join(dir, "console.json")
	require.NoError(t, os.WriteFile(artifact, []byte(`{"messages":["ok"]}`), 0o644))

	return Manifest{
		SchemaVersion:       SchemaVersion,
		QAResultID:          "qa-browser-login-001",
		Surface:             surface,
		Lane:                "golden",
		ScenarioRef:         "browser:login",
		Runner:              Runner{Name: "playwright", Command: "npx playwright test e2e/tests/qamesh-golden.spec.ts --project=chromium"},
		Status:              status,
		StartedAt:           "2026-05-02T00:00:00Z",
		EndedAt:             "2026-05-02T00:00:01Z",
		DurationMS:          1000,
		RetentionClass:      "local-redacted",
		ReproductionCommand: "PLAYWRIGHT_SKIP_GLOBAL_SETUP=true npx playwright test e2e/tests/qamesh-golden.spec.ts --project=chromium",
		Artifacts: []ArtifactRef{{
			Kind:        "console",
			Path:        artifact,
			Publishable: true,
			Redaction:   "text_redacted_and_scanned",
		}},
		OracleResults: OracleResults{
			A11y: &A11yOracle{CriticalCount: 1, FailedTargets: []string{`button[name="Pay"]`}},
		},
		RedactionStatus: RedactionStatus{Status: "passed"},
		SourceRefs: SourceRefs{
			SourceSpec:       "SPEC-QAMESH-001",
			AcceptanceRefs:   []string{"AC-QAMESH-001"},
			OwnedPaths:       []string{"Autopus/frontend"},
			DoNotModifyPaths: []string{".codex/**"},
		},
	}
}
