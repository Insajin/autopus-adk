package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidateAcceptsMobileV2Contract(t *testing.T) {
	t.Parallel()

	manifest := fixtureMobileManifest(t)

	require.NoError(t, manifest.Validate())
}

func TestWriteFinalManifestBlocksUnsafeMobileArtifact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := fixtureMobileManifest(t)
	logPath := filepath.Join(dir, "mobile.log")
	require.NoError(t, os.WriteFile(logPath, []byte("UDID=00008110ABCDEF123456789000008110ABCDEF12"), 0o644))
	manifest.Artifacts = []ArtifactRef{{
		Kind:        "sanitized_log",
		Path:        logPath,
		Publishable: true,
		Redaction:   "text_redacted_and_scanned",
	}}

	_, err := WriteFinalManifest(manifest, filepath.Join(dir, "final"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe_mobile_artifact")
	assert.NoDirExists(t, filepath.Join(dir, "final"))
}

func TestManifestValidateRejectsInvalidMobileDigest(t *testing.T) {
	t.Parallel()

	manifest := fixtureMobileManifest(t)
	manifest.SourceRefs.Mobile.AppArtifactDigest = "app-debug.apk"

	err := manifest.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_artifact_digest")
}

func TestManifestValidateRejectsPersonalMobileDeviceRef(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{"Alice iPhone 15", "device-ref:alice-iphone"} {
		manifest := fixtureMobileManifest(t)
		manifest.SourceRefs.Mobile.DeviceRef = ref

		err := manifest.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "device_ref")
	}
}

func TestManifestValidateRequiresLocalOnlyMobileQuarantineRef(t *testing.T) {
	t.Parallel()

	manifest := fixtureMobileManifest(t)
	manifest.Artifacts[1].Publishable = true

	err := manifest.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "quarantine")
}

func TestWriteFinalManifestBlocksSignedURLMobileQuarantineRef(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := fixtureMobileManifest(t)
	refPath := filepath.Join(dir, "screenshot-ref.json")
	require.NoError(t, os.WriteFile(refPath, []byte(`{"url":"https://example.test/mobile.png?sig=private"}`), 0o644))
	manifest.Artifacts = []ArtifactRef{{
		Kind:        "screenshot_quarantine_ref",
		Path:        refPath,
		Publishable: false,
		Redaction:   "local_only_quarantine_ref",
	}}

	_, err := WriteFinalManifest(manifest, filepath.Join(dir, "final"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe_mobile_artifact")
	assert.NoDirExists(t, filepath.Join(dir, "final"))
}

func fixtureMobileManifest(t *testing.T) Manifest {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
		return path
	}
	return Manifest{
		SchemaVersion:       SchemaVersionV2,
		QAResultID:          "qa-mobile-001",
		Surface:             "mobile",
		Lane:                "mobile-readiness",
		ScenarioRef:         "mobile:login",
		Runner:              Runner{Name: "maestro-scripted", Command: "maestro test .autopus/qa/mobile/flows/login.yaml"},
		Status:              "passed",
		StartedAt:           "2026-05-15T00:00:00Z",
		EndedAt:             "2026-05-15T00:00:01Z",
		DurationMS:          1000,
		RetentionClass:      "local-redacted",
		ReproductionCommand: "maestro test .autopus/qa/mobile/flows/login.yaml",
		Artifacts: []ArtifactRef{
			{Kind: "sanitized_log", Path: write("mobile.log", "ok\n"), Publishable: true, Redaction: "text_redacted_and_scanned"},
			{Kind: "screenshot_quarantine_ref", Path: write("screenshot-ref.json", `{"ref":"local-quarantine"}`), Publishable: false, Redaction: "local_only_quarantine_ref"},
		},
		OracleResults: OracleResults{Checks: []CheckResult{{
			ID:       "login-visible",
			Type:     "mobile_check",
			Status:   "passed",
			Expected: "login screen visible",
			Actual:   "visible",
		}}},
		RedactionStatus: RedactionStatus{Status: "passed"},
		SourceRefs: SourceRefs{
			SourceSpec:       "SPEC-QAMESH-006",
			AcceptanceRefs:   []string{"AC-QAMESH6-008"},
			OwnedPaths:       []string{".autopus/qa/mobile/**"},
			DoNotModifyPaths: []string{".codex/**"},
			JourneyID:        "mobile-login",
			StepID:           "step-1",
			Adapter:          "maestro-scripted",
			Mobile: &MobileRefs{
				FlowID:            "flow-login",
				AppArtifactDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				DeviceRef:         "device-ref:ios-sim",
			},
		},
	}
}
