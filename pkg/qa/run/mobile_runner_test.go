package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/mobile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLookupDeviceHandle covers the device-ref-to-handle JSON map parser.
func TestLookupDeviceHandle(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		deviceRef  string
		wantHandle string
		wantOK     bool
	}{
		{"valid hit", `{"device-ref:x":"handle-1"}`, "device-ref:x", "handle-1", true},
		{"empty raw", "", "device-ref:x", "", false},
		{"malformed json", `{not json`, "device-ref:x", "", false},
		{"key absent", `{"device-ref:y":"handle-2"}`, "device-ref:x", "", false},
		{"empty handle value", `{"device-ref:x":""}`, "device-ref:x", "", false},
		{"empty device ref", `{"device-ref:x":"handle-1"}`, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle, ok := lookupDeviceHandle(tt.raw, tt.deviceRef)
			assert.Equal(t, tt.wantHandle, handle)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

// TestRealMobileDeviceRunnerResolveEnvPath resolves via the env-based device map.
func TestRealMobileDeviceRunnerResolveEnvPath(t *testing.T) {
	t.Setenv(mobileDeviceMapEnv, `{"device-ref:x":"handle-1"}`)
	handle, ok := realMobileDeviceRunner{}.Resolve(mobileResolveRequest{DeviceRef: "device-ref:x"})
	assert.True(t, ok)
	assert.Equal(t, "handle-1", handle)
}

// TestRealMobileDeviceRunnerResolveFilePath resolves via devices.local.json when env is absent.
func TestRealMobileDeviceRunnerResolveFilePath(t *testing.T) {
	t.Setenv(mobileDeviceMapEnv, "")
	tmp := t.TempDir()
	mapPath := filepath.Join(tmp, ".autopus", "qa", "mobile", "devices.local.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(mapPath), 0o755))
	require.NoError(t, os.WriteFile(mapPath, []byte(`{"device-ref:y":"handle-2"}`), 0o644))

	handle, ok := realMobileDeviceRunner{}.Resolve(mobileResolveRequest{DeviceRef: "device-ref:y", ProjectDir: tmp})
	assert.True(t, ok)
	assert.Equal(t, "handle-2", handle)
}

// TestRealMobileDeviceRunnerResolveMiss returns false when neither env nor file resolve.
func TestRealMobileDeviceRunnerResolveMiss(t *testing.T) {
	t.Setenv(mobileDeviceMapEnv, "")
	tmp := t.TempDir()
	handle, ok := realMobileDeviceRunner{}.Resolve(mobileResolveRequest{DeviceRef: "device-ref:z", ProjectDir: tmp})
	assert.False(t, ok)
	assert.Equal(t, "", handle)
}

// TestRealMobileDeviceRunnerResolveEnvPrecedence prefers env over file when both define the ref.
func TestRealMobileDeviceRunnerResolveEnvPrecedence(t *testing.T) {
	t.Setenv(mobileDeviceMapEnv, `{"device-ref:x":"env-handle"}`)
	tmp := t.TempDir()
	mapPath := filepath.Join(tmp, ".autopus", "qa", "mobile", "devices.local.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(mapPath), 0o755))
	require.NoError(t, os.WriteFile(mapPath, []byte(`{"device-ref:x":"file-handle"}`), 0o644))

	handle, ok := realMobileDeviceRunner{}.Resolve(mobileResolveRequest{DeviceRef: "device-ref:x", ProjectDir: tmp})
	assert.True(t, ok)
	assert.Equal(t, "env-handle", handle)
}

// TestRealMobileDeviceRunnerRunFlow exercises the real RunFlow -> runCommandWithEnv
// path with a benign fake binary, asserting passed status and that stdout.log is
// written into ArtifactDir (no real device involved).
func TestRealMobileDeviceRunnerRunFlow(t *testing.T) {
	bin := installFakeMobileBinary(t, "fake-flow", "echo flow ran")
	tmp := t.TempDir()
	artifactDir := filepath.Join(tmp, "artifacts")
	pack := journey.Pack{
		ID:      "mobile-runflow",
		Surface: "mobile",
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
		Command: journey.Command{Argv: []string{bin}, CWD: ".", Timeout: "30s"},
	}

	result := realMobileDeviceRunner{}.RunFlow(context.Background(), mobileFlowRequest{
		ProjectDir:  tmp,
		Pack:        pack,
		Handle:      "emulator-5554",
		ArtifactDir: artifactDir,
	})

	assert.Equal(t, "passed", result.Status)
	assert.FileExists(t, filepath.Join(artifactDir, "stdout.log"))
	stdout, err := os.ReadFile(filepath.Join(artifactDir, "stdout.log"))
	require.NoError(t, err)
	assert.Contains(t, string(stdout), "flow ran")
}

// TestRealMobileDeviceRunnerInstallApp covers the fail-closed install branches
// when platform tools are absent from PATH. No real install is attempted.
func TestRealMobileDeviceRunnerInstallApp(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	ctx := context.Background()

	androidErr := realMobileDeviceRunner{}.InstallApp(ctx, mobileInstallRequest{Platform: "android", AppPath: "app.apk"})
	assert.Error(t, androidErr)

	iosErr := realMobileDeviceRunner{}.InstallApp(ctx, mobileInstallRequest{Platform: "ios", Handle: "sim", AppPath: "app.ipa"})
	assert.Error(t, iosErr)

	defaultErr := realMobileDeviceRunner{}.InstallApp(ctx, mobileInstallRequest{Platform: "", AppPath: "app"})
	assert.NoError(t, defaultErr)
}

// TestProbeMobileToolsMaestroAbsent surfaces a setup gap mentioning maestro.
func TestProbeMobileToolsMaestroAbsent(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	gap := probeMobileTools("android")
	require.NotNil(t, gap)
	assert.Contains(t, gap.Reason, "maestro")
}

// TestProbeMobileToolsDriverAbsent surfaces a setup gap mentioning the platform driver
// (adb) when maestro is present but the android driver is missing.
func TestProbeMobileToolsDriverAbsent(t *testing.T) {
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "maestro"), []byte("#!/bin/sh\necho ok\n"), 0o755))
	t.Setenv("PATH", binDir)
	gap := probeMobileTools("android")
	require.NotNil(t, gap)
	assert.Contains(t, gap.Reason, "adb")
}

// TestFirstPlatform returns the first non-empty platform from device inventory,
// then simulator/emulator targets, then "".
func TestFirstPlatform(t *testing.T) {
	devicePlatform := mobile.Readiness{}
	devicePlatform.DeviceInventory.Devices = []mobile.DeviceTarget{{DeviceRef: "device-ref:a", Platform: "android"}}
	assert.Equal(t, "android", firstPlatform(devicePlatform))

	targetPlatform := mobile.Readiness{}
	targetPlatform.SimulatorEmulator.Targets = []mobile.DeviceTarget{{TargetRef: "sim-ref:b", Platform: "ios"}}
	assert.Equal(t, "ios", firstPlatform(targetPlatform))

	assert.Equal(t, "", firstPlatform(mobile.Readiness{}))
}

// TestComputeFileDigest returns sha256:<hex> for a known file and an error for a
// nonexistent path.
func TestComputeFileDigest(t *testing.T) {
	tmp := t.TempDir()
	content := []byte("hello mobile\n")
	path := filepath.Join(tmp, "app.bin")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	sum := sha256.Sum256(content)
	want := "sha256:" + hex.EncodeToString(sum[:])

	got, err := computeFileDigest(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = computeFileDigest(filepath.Join(tmp, "does-not-exist.bin"))
	assert.Error(t, err)
}
