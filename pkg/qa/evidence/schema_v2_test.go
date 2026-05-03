package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest_PreservesV2GenericChecksAndThresholds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := fixtureV2Manifest(t, "cli")
	path := filepath.Join(dir, "manifest.json")
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, body, 0o644))

	loaded, err := LoadManifest(path)

	require.NoError(t, err)
	require.NoError(t, loaded.Validate())
	assert.Equal(t, SchemaVersionV2, loaded.SchemaVersion)
	assert.Equal(t, "npm-test", loaded.OracleResults.Checks[0].ID)
	assert.Equal(t, float64(0), loaded.SourceRefs.OracleThresholds["exit_code"])
}

func TestLoadManifest_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o644))

	_, err := LoadManifest(path)

	require.ErrorContains(t, err, "parse manifest")
}

func TestWriteFinalManifest_WritesV2GenericEvidenceWithoutLegacyOracles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := fixtureV2Manifest(t, "package")
	manifest.OracleResults.A11y = nil
	manifest.OracleResults.Desktop = nil

	manifestPath, err := WriteFinalManifest(manifest, filepath.Join(dir, "final"))

	require.NoError(t, err)
	body, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, `"schema_version": "qamesh.evidence.v2"`)
	assert.Contains(t, text, `"checks": [`)
	assert.Contains(t, text, `"journey_id": "journey-node"`)
	assert.NotContains(t, text, `"a11y"`)
	assert.NotContains(t, text, `"desktop"`)
	assert.FileExists(t, filepath.Join(dir, "final", "artifacts", "stdout", "stdout.log"))
}

func TestManifestValidate_RejectsV2InvalidCheckFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*CheckResult)
		wantErr string
	}{
		{
			name:    "missing id",
			mutate:  func(check *CheckResult) { check.ID = "" },
			wantErr: "oracle_results.checks[0].id",
		},
		{
			name:    "missing type",
			mutate:  func(check *CheckResult) { check.Type = "" },
			wantErr: "oracle_results.checks[0].type",
		},
		{
			name:    "unsupported status",
			mutate:  func(check *CheckResult) { check.Status = "warning" },
			wantErr: "oracle_results.checks[0].status",
		},
		{
			name: "missing failed summary",
			mutate: func(check *CheckResult) {
				check.Status = "failed"
				check.FailureSummary = ""
			},
			wantErr: "oracle_results.checks[0].failure_summary",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manifest := fixtureV2Manifest(t, "package")
			tt.mutate(&manifest.OracleResults.Checks[0])

			require.ErrorContains(t, manifest.Validate(), tt.wantErr)
		})
	}
}

func TestManifestValidate_RejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")
	manifest.SchemaVersion = "qamesh.evidence.v3"

	require.ErrorContains(t, manifest.Validate(), "unsupported schema_version")
}

func TestNormalizeManifest_GenericBlockedCheckSetsBlockedStatus(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")
	manifest.Status = "passed"
	manifest.OracleResults.Checks[0].Status = "blocked"
	manifest.OracleResults.Checks[0].FailureSummary = "timeout after 60s"

	normalized := NormalizeManifest(manifest)

	assert.Equal(t, "blocked", normalized.Status)
}

func TestWriteFeedbackBundle_RequiresOwnedAndDoNotModifyPathsForV2(t *testing.T) {
	t.Parallel()

	manifest := fixtureV2Manifest(t, "package")
	manifest.SourceRefs.OwnedPaths = nil

	_, err := WriteFeedbackBundle(manifest, "codex", t.TempDir())

	require.ErrorContains(t, err, "owned_paths")
}
