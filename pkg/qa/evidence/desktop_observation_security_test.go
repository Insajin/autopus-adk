package evidence

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopObservationProfile_AnyQ12MarkerRejectsStrippedTypedContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		marker func(*Manifest)
	}{
		{name: "source spec", marker: func(manifest *Manifest) { manifest.SourceRefs.SourceSpec = "SPEC-QAMESH-012" }},
		{name: "adapter", marker: func(manifest *Manifest) { manifest.SourceRefs.Adapter = "desktop-accessibility-observe" }},
		{name: "journey", marker: func(manifest *Manifest) { manifest.SourceRefs.JourneyID = "desktop-accessibility-observe" }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			manifest := fixtureV2Manifest(t, "desktop")
			rawAX := filepath.Join(dir, "raw-ax.json")
			require.NoError(t, os.WriteFile(rawAX, []byte(`{"raw_ax":{"pid":42}}`), 0o600))
			manifest.Artifacts[0] = ArtifactRef{
				Kind: "stdout", Path: rawAX, Publishable: true, Redaction: "text_redacted_and_scanned",
			}
			test.marker(&manifest)
			output := filepath.Join(dir, "published")

			_, err := WriteFinalManifest(manifest, output)

			require.Error(t, err)
			assert.NoDirExists(t, output)
		})
	}
}

func TestDesktopObservationProfile_RejectsFreeFormPublicationChannels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "runner name", mutate: func(manifest *Manifest) { manifest.Runner.Name = "provider_dump" }},
		{name: "runner command", mutate: func(manifest *Manifest) { manifest.Runner.Command = "provider_dump" }},
		{name: "runner version", mutate: func(manifest *Manifest) { manifest.Runner.Version = "provider_dump" }},
		{name: "check expected", mutate: func(manifest *Manifest) { manifest.OracleResults.Checks[0].Expected = "provider_dump" }},
		{name: "check actual", mutate: func(manifest *Manifest) { manifest.OracleResults.Checks[0].Actual = "provider_dump" }},
		{name: "check failure summary", mutate: func(manifest *Manifest) { manifest.OracleResults.Checks[0].FailureSummary = "provider_dump" }},
		{name: "check artifact traversal", mutate: func(manifest *Manifest) {
			manifest.OracleResults.Checks[0].ArtifactRefs = []string{"../../provider_dump"}
		}},
		{name: "acceptance ref", mutate: func(manifest *Manifest) { manifest.SourceRefs.AcceptanceRefs = []string{"provider_dump"} }},
		{name: "journey step", mutate: func(manifest *Manifest) { manifest.SourceRefs.StepID = "provider_dump" }},
		{name: "owned paths", mutate: func(manifest *Manifest) { manifest.SourceRefs.OwnedPaths = []string{"provider_dump/**"} }},
		{name: "do not modify paths", mutate: func(manifest *Manifest) {
			manifest.SourceRefs.DoNotModifyPaths = []string{"provider_dump/**"}
		}},
		{name: "oracle thresholds", mutate: func(manifest *Manifest) {
			manifest.SourceRefs.OracleThresholds = map[string]any{"provider_dump": "bytes"}
		}},
		{name: "mobile refs", mutate: func(manifest *Manifest) {
			manifest.SourceRefs.Mobile = &MobileRefs{FlowID: "provider_dump", AppArtifactDigest: "bytes", DeviceRef: "device"}
		}},
		{name: "reproduction command", mutate: func(manifest *Manifest) { manifest.ReproductionCommand = "provider_dump" }},
		{name: "repair prompt ref", mutate: func(manifest *Manifest) { manifest.RepairPromptRef = "provider_dump" }},
		{name: "redaction findings", mutate: func(manifest *Manifest) {
			manifest.RedactionStatus.Findings = []Finding{{Type: "provider_dump", Source: "provider_dump", Sample: "bytes"}}
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
			test.mutate(&manifest)
			output := filepath.Join(dir, "published")

			require.Error(t, manifest.Validate())
			_, err := WriteFinalManifest(manifest, output)

			require.Error(t, err)
			assert.NoDirExists(t, output)
		})
	}
}

func TestDesktopObservationProfile_MarkerDetectionIsFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest Manifest
		want     bool
	}{
		{name: "legacy lane is not marker", manifest: Manifest{SchemaVersion: SchemaVersionV1, Lane: desktopObservationLane}},
		{name: "generic v2 desktop native lane is not marker", manifest: Manifest{SchemaVersion: SchemaVersionV2, Lane: desktopObservationLane}},
		{name: "runner", manifest: Manifest{SchemaVersion: SchemaVersionV2, Runner: Runner{Name: desktopObservationAdapterID}}, want: true},
		{name: "check id", manifest: Manifest{SchemaVersion: SchemaVersionV2, OracleResults: OracleResults{Checks: []CheckResult{{ID: desktopobserve.DeterministicCheckSemanticLandmarks}}}}, want: true},
		{name: "check type", manifest: Manifest{SchemaVersion: SchemaVersionV2, OracleResults: OracleResults{Checks: []CheckResult{{Type: desktopObservationCheckType}}}}, want: true},
		{name: "artifact", manifest: Manifest{SchemaVersion: SchemaVersionV2, Artifacts: []ArtifactRef{{Kind: desktopObservationArtifact}}}, want: true},
		{name: "acceptance", manifest: Manifest{SchemaVersion: SchemaVersionV2, SourceRefs: SourceRefs{AcceptanceRefs: []string{"AC-QAMESH12-018"}}}, want: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, isDesktopObservationContract(test.manifest))
		})
	}
}

func TestDesktopObservationProfile_GenericV2DesktopNativeRemainsCompatible(t *testing.T) {
	t.Parallel()
	manifest := fixtureV2Manifest(t, "desktop")
	manifest.Lane, manifest.ScenarioRef = desktopObservationLane, "desktop:generic"
	manifest.Runner = Runner{Name: "custom-command", Command: "false"}
	manifest.SourceRefs.SourceSpec, manifest.SourceRefs.JourneyID = "SPEC-QAMESH-011", "generic-desktop"
	manifest.SourceRefs.AcceptanceRefs = []string{"AC-QAMESH11-009"}
	manifest.SourceRefs.Adapter = "custom-command"
	path, err := WriteFinalManifest(manifest, filepath.Join(t.TempDir(), "published"))
	require.NoError(t, err)
	assert.FileExists(t, path)
}

func TestDesktopObservationProfile_AcceptanceRefsUseBoundedAllowlist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		refs []string
		want bool
	}{
		{name: "missing"},
		{name: "wrong prefix", refs: []string{"AC-OTHER12-001"}},
		{name: "non numeric", refs: []string{"AC-QAMESH12-ABC"}},
		{name: "below range", refs: []string{"AC-QAMESH12-000"}},
		{name: "above range", refs: []string{"AC-QAMESH12-019"}},
		{name: "duplicate", refs: []string{"AC-QAMESH12-001", "AC-QAMESH12-001"}},
		{name: "allowed", refs: []string{"AC-QAMESH12-001", "AC-QAMESH12-018"}, want: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, validDesktopObservationAcceptanceRefs(test.refs))
		})
	}
}

func TestLoadManifest_RejectsDuplicateKeysAcrossWholeDocument(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	body = bytes.Replace(body, []byte(`{"schema_version":`), []byte(`{"schema_version":"qamesh.evidence.v2","schema_version":`), 1)
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	_, err = LoadManifest(path)

	require.Error(t, err)
	assert.ErrorIs(t, err, desktopobserve.ErrDuplicateKey)
}

func TestLoadManifest_Q12ProfileIsStrictAndValidated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   error
	}{
		{name: "top level unknown", mutate: func(body []byte) []byte {
			return bytes.Replace(body, []byte(`{"schema_version":`), []byte(`{"provider_dump":"bytes","schema_version":`), 1)
		}, want: desktopobserve.ErrUnknownField},
		{name: "nested raw payload", mutate: func(body []byte) []byte {
			return bytes.Replace(body, []byte(`"provider_ref":"provider-local"`), []byte(`"provider_ref":"provider-local","raw_payload":"bytes"`), 1)
		}, want: desktopobserve.ErrUnknownField},
		{name: "nested duplicate", mutate: func(body []byte) []byte {
			return bytes.Replace(body, []byte(`"provider_ref":"provider-local"`), []byte(`"provider_ref":"provider-local","provider_ref":"provider-local"`), 1)
		}, want: desktopobserve.ErrDuplicateKey},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
			body, err := json.Marshal(manifest)
			require.NoError(t, err)
			body = test.mutate(body)
			path := filepath.Join(dir, "manifest.json")
			require.NoError(t, os.WriteFile(path, body, 0o600))

			_, err = LoadManifest(path)

			require.Error(t, err)
			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestLoadManifest_Q12MarkerIncompleteDocumentFailsClosed(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "desktop")
	manifest.SourceRefs.SourceSpec = "SPEC-QAMESH-012"
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	_, err = LoadManifest(path)

	require.Error(t, err)
}

func TestLoadManifest_NullInlineQ12MarkerFailsClosed(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "desktop")
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	body = bytes.Replace(
		body,
		[]byte(`"oracle_results":{`),
		[]byte(`"oracle_results":{"desktop_observation":null,`),
		1,
	)
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	_, err = LoadManifest(path)

	require.Error(t, err)
}

func TestLoadManifest_GenericV1AndV2UnknownExtensionsRemainCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest Manifest
	}{
		{name: "generic v2 desktop", manifest: fixtureV2Manifest(t, "desktop")},
		{name: "legacy v1", manifest: legacyDesktopObservationManifest()},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body, err := json.Marshal(test.manifest)
			require.NoError(t, err)
			body = bytes.Replace(body, []byte(`{"schema_version":`), []byte(`{"extension":{"provider_dump":"generic-compatible"},"schema_version":`), 1)
			path := filepath.Join(t.TempDir(), "manifest.json")
			require.NoError(t, os.WriteFile(path, body, 0o600))

			loaded, err := LoadManifest(path)

			require.NoError(t, err)
			assert.Equal(t, test.manifest.SchemaVersion, loaded.SchemaVersion)
		})
	}
}

func legacyDesktopObservationManifest() Manifest {
	return Manifest{
		SchemaVersion: SchemaVersionV1, QAResultID: "legacy-desktop", Surface: "desktop",
		Lane: "desktop-native", ScenarioRef: "desktop:legacy", Runner: Runner{Name: "appium"},
		Status: "passed", StartedAt: "2026-07-18T00:00:00Z", EndedAt: "2026-07-18T00:00:01Z",
		RetentionClass: "local-redacted",
		Artifacts: []ArtifactRef{
			{Kind: "screenshot", Path: "shot.json"}, {Kind: "app_log", Path: "app.log"},
			{Kind: "driver_log", Path: "driver.log"}, {Kind: "command_output", Path: "command.log"},
		},
		OracleResults:   OracleResults{Desktop: &DesktopOracle{TimeoutClassification: "none"}},
		RedactionStatus: RedactionStatus{Status: "passed"},
		SourceRefs: SourceRefs{
			SourceSpec: "SPEC-DESKTOP-017", AcceptanceRefs: []string{"AC-DESKTOP17-001"},
		},
	}
}
