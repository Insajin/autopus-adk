package mobile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssessMissingReadinessReportsSetupGaps(t *testing.T) {
	t.Parallel()

	readiness := Assess(t.TempDir())

	assert.Equal(t, StatusSetupGap, readiness.Status)
	assert.Equal(t, StatusMissing, readiness.DeviceInventory.Status)
	assert.Equal(t, StatusMissing, readiness.SimulatorEmulator.Status)
	assert.Equal(t, StatusMissing, readiness.AppArtifact.Status)
	assert.Equal(t, StatusMissing, readiness.Credentials.Status)
	assert.Equal(t, "passed", readiness.RedactionStatus.Status)
	assert.Empty(t, readiness.SideEffects)
	assert.Contains(t, setupGapCodes(readiness.SetupGaps), ReasonMissingDeviceInventory)
	assert.Contains(t, setupGapCodes(readiness.SetupGaps), ReasonMissingAppArtifact)
}

func TestAssessCloudLabWithoutPoliciesIsDeferred(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, `
cloud_lab:
  provider: browserstack
  opt_in: false
`)

	readiness := Assess(dir)

	assert.Equal(t, StatusDeferred, readiness.Status)
	assert.Equal(t, StatusDeferred, readiness.CloudLab.Status)
	assert.False(t, readiness.CloudLab.OptIn)
	assert.Contains(t, setupGapCodes(readiness.SetupGaps), ReasonCloudLabPolicyIncomplete)
}

func TestAssessBlocksUnsafePlanningOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, `
app_artifact:
  path: /Users/alice/private/app.ipa
  digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
credentials:
  refs:
    - sk-proj-qameshfake1234567890
device_inventory:
  devices:
    - device_ref: 00008110ABCDEF123456789000008110ABCDEF12
      platform: ios
`)

	readiness := Assess(dir)

	assert.Equal(t, "blocked", readiness.RedactionStatus.Status)
	assert.NotContains(t, readiness.AppArtifact.Path, "/Users/alice")
	assert.NotContains(t, readiness.Credentials.Refs, "sk-proj")
	assert.NotContains(t, readiness.DeviceInventory.Devices[0].DeviceRef, "00008110")
	assert.Contains(t, findingTypes(readiness.RedactionStatus.Findings), "local_user_path")
	assert.Contains(t, findingTypes(readiness.RedactionStatus.Findings), "credential_ref")
	assert.Contains(t, findingTypes(readiness.RedactionStatus.Findings), "device_identifier")
}

func TestAssessReadyLocalReadinessUsesSafeRefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	appPath := filepath.Join(dir, ".autopus", "qa", "mobile", "apps", "app.ipa")
	require.NoError(t, os.MkdirAll(filepath.Dir(appPath), 0o755))
	require.NoError(t, os.WriteFile(appPath, []byte("fake app"), 0o644))
	writeConfig(t, dir, `
device_inventory:
  devices:
    - device_ref: device-ref:ios-sim
      platform: ios
simulator_emulator:
  targets:
    - target_ref: simulator-ref:ios-17
      platform: ios
app_artifact:
  path: .autopus/qa/mobile/apps/app.ipa
  digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
credentials:
  refs:
    - credential-ref:ios-dev
`)

	readiness := Assess(dir)

	assert.Equal(t, StatusReady, readiness.Status)
	assert.Equal(t, StatusReady, readiness.AppArtifact.Status)
	assert.Empty(t, readiness.SetupGaps)
	assert.Equal(t, "passed", readiness.RedactionStatus.Status)
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, ".autopus", "qa", "mobile", "readiness.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func setupGapCodes(gaps []SetupGap) []string {
	codes := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		codes = append(codes, gap.ReasonCode)
	}
	return codes
}

func findingTypes(findings []Finding) []string {
	types := make([]string, 0, len(findings))
	for _, finding := range findings {
		types = append(types, finding.Type)
	}
	return types
}
