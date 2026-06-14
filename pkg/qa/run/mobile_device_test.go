package run

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/mobile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteMobilePackReportsUnresolvedDeviceRef(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	runDir := filepath.Join(dir, "runs", "qa-unresolved")
	fake := &fakeMobileDeviceRunner{handles: map[string]string{}}
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")

	result, _, _ := executeMobilePack(Options{ProjectDir: dir, Lane: "mobile-scripted", deviceRunner: fake}, pack, filepath.Join(runDir, "_raw"), runDir)

	require.NotNil(t, result.SetupGap)
	assert.Contains(t, result.SetupGap.Reason, mobile.ReasonDeviceRefUnresolved)
	assert.Equal(t, 0, fake.FlowCalls)
}

func TestExecuteMobilePackBlocksDigestMismatchBeforeInstall(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	appPath := filepath.Join(dir, ".autopus", "qa", "mobile", "apps", "app.apk")
	require.NoError(t, os.MkdirAll(filepath.Dir(appPath), 0o755))
	require.NoError(t, os.WriteFile(appPath, []byte("real bytes differ from declared digest"), 0o644))
	runDir := filepath.Join(dir, "runs", "qa-mismatch")
	fake := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}}
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")

	result, _, _ := executeMobilePack(Options{ProjectDir: dir, Lane: "mobile-scripted", ManagedDevice: true, deviceRunner: fake}, pack, filepath.Join(runDir, "_raw"), runDir)

	require.NotNil(t, result.SetupGap)
	assert.Contains(t, result.SetupGap.Reason, mobile.ReasonAppArtifactDigestMismatch)
	assert.Equal(t, 0, fake.InstallCalls)
	assert.Equal(t, 0, fake.FlowCalls)
}

func TestExecuteMobilePackBlocksWhenMaestroToolMissing(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	appPath := filepath.Join(dir, ".autopus", "qa", "mobile", "apps", "app.apk")
	require.NoError(t, os.MkdirAll(filepath.Dir(appPath), 0o755))
	content := []byte("managed app bytes")
	require.NoError(t, os.WriteFile(appPath, content, 0o644))
	sum := sha256.Sum256(content)
	emptyBin := filepath.Join(dir, "emptybin")
	require.NoError(t, os.MkdirAll(emptyBin, 0o755))
	t.Setenv("PATH", emptyBin)
	runDir := filepath.Join(dir, "runs", "qa-tools")
	fake := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}}
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")
	pack.Mobile.AppArtifactDigest = "sha256:" + hex.EncodeToString(sum[:])

	result, _, _ := executeMobilePack(Options{ProjectDir: dir, Lane: "mobile-scripted", ManagedDevice: true, deviceRunner: fake}, pack, filepath.Join(runDir, "_raw"), runDir)

	require.NotNil(t, result.SetupGap)
	assert.Contains(t, result.SetupGap.Reason, "maestro")
	assert.Equal(t, 0, fake.FlowCalls)
}
