package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopObservationConsistency_PassedManifestRejectsContradictoryTypedEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "semantic projection absent", mutate: func(manifest *Manifest) {
			manifest.OracleResults.DesktopObservation.SemanticProjection = nil
		}},
		{name: "receipt has failure reason", mutate: func(manifest *Manifest) {
			reason := desktopobserve.ReasonProviderUnavailable
			next := desktopobserve.NextStep(reason)
			receipt := &manifest.OracleResults.DesktopObservation.RuntimeReceipt
			receipt.ReasonCode, receipt.NextStep = &reason, &next
		}},
		{name: "receipt redaction failed", mutate: func(manifest *Manifest) {
			manifest.OracleResults.DesktopObservation.RuntimeReceipt.Redaction.Status = desktopobserve.RedactionFailed
		}},
		{name: "receipt quarantine blocked", mutate: func(manifest *Manifest) {
			manifest.OracleResults.DesktopObservation.RuntimeReceipt.Quarantine.Status = desktopobserve.QuarantineBlocked
		}},
		{name: "deterministic observation check blocked", mutate: func(manifest *Manifest) {
			manifest.OracleResults.DesktopObservation.DeterministicChecks[0].Status = desktopobserve.CheckBlocked
		}},
		{name: "manifest check blocked", mutate: func(manifest *Manifest) {
			manifest.OracleResults.Checks[0].Status = "blocked"
			manifest.OracleResults.Checks[0].FailureSummary = "blocked by typed oracle"
		}},
		{name: "digest tampered", mutate: func(manifest *Manifest) {
			manifest.OracleResults.DesktopObservation.SemanticProjection.Digest = strings.Repeat("0", 64)
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
			test.mutate(&manifest)
			syncDesktopObservationArtifact(t, manifest)
			output := filepath.Join(dir, "published")
			_, err := WriteFinalManifest(manifest, output)
			require.Error(t, err)
			assert.NoDirExists(t, output)
		})
	}
}

func TestDesktopObservationConsistency_InlineArtifactMismatchFailsClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
	body, err := os.ReadFile(manifest.Artifacts[0].Path)
	require.NoError(t, err)
	body = []byte(strings.Replace(string(body), `"name":"Autopus"`, `"name":"Other"`, 1))
	require.NoError(t, os.WriteFile(manifest.Artifacts[0].Path, body, 0o600))
	output := filepath.Join(dir, "published")

	_, err = WriteFinalManifest(manifest, output)
	require.Error(t, err)
	assert.NoDirExists(t, output)
}

func TestDesktopObservationConsistency_NonPassManifestCannotCarryPassingOracle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "failed")
	manifest.OracleResults.Checks[0].Status = "passed"
	manifest.OracleResults.Checks[0].FailureSummary = ""
	output := filepath.Join(dir, "published")

	_, err := WriteFinalManifest(manifest, output)
	require.Error(t, err)
	assert.NoDirExists(t, output)
}

func syncDesktopObservationArtifact(t *testing.T, manifest Manifest) {
	t.Helper()
	body, err := json.Marshal(manifest.OracleResults.DesktopObservation)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifest.Artifacts[0].Path, body, 0o600))
}
