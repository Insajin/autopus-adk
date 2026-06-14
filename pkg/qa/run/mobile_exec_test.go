package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMobileDeviceRunner is a white-box stub for MobileDeviceRunner (contract 1.3).
// Resolve maps device_ref -> handle; RunFlow records the handle it received and
// writes stdout.log/stderr.log into the request ArtifactDir so the manifest's
// sanitized_log refs resolve.
type fakeMobileDeviceRunner struct {
	handles    map[string]string
	flowExit   int
	installErr error

	ResolveCalls int
	InstallCalls int
	FlowCalls    int
	LastHandle   string
}

func (f *fakeMobileDeviceRunner) Resolve(req mobileResolveRequest) (string, bool) {
	f.ResolveCalls++
	handle, ok := f.handles[req.DeviceRef]
	return handle, ok
}

func (f *fakeMobileDeviceRunner) InstallApp(ctx context.Context, req mobileInstallRequest) error {
	f.InstallCalls++
	return f.installErr
}

func (f *fakeMobileDeviceRunner) RunFlow(ctx context.Context, req mobileFlowRequest) commandResult {
	f.FlowCalls++
	f.LastHandle = req.Handle
	_ = os.MkdirAll(req.ArtifactDir, 0o755)
	stdout := filepath.Join(req.ArtifactDir, "stdout.log")
	stderr := filepath.Join(req.ArtifactDir, "stderr.log")
	_ = os.WriteFile(stdout, []byte("flow ok\n"), 0o644)
	_ = os.WriteFile(stderr, []byte(""), 0o644)
	status := "passed"
	summary := ""
	if f.flowExit != 0 {
		status = "failed"
		summary = "maestro flow exited non-zero"
	}
	now := time.Now().UTC()
	return commandResult{
		Status:         status,
		ExitCode:       f.flowExit,
		FailureSummary: summary,
		StdoutPath:     stdout,
		StderrPath:     stderr,
		StartedAt:      now,
		EndedAt:        now.Add(time.Second),
		DurationMS:     1000,
		Command:        "maestro test " + req.Pack.Mobile.FlowPath,
	}
}

func mobileScriptedJourneyPack(flowPath string) journey.Pack {
	return journey.Pack{
		ID:      "mobile-scripted-smoke",
		Surface: "mobile",
		Lanes:   []string{"mobile-scripted"},
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
		Command: journey.Command{Argv: []string{"maestro", "test", flowPath}, CWD: ".", Timeout: "120s"},
		Checks:  []journey.Check{{ID: "mobile-scripted-smoke", Type: "deterministic", Expected: map[string]any{"exit_code": 0}}},
		Mobile: journey.MobilePolicy{
			FlowPath:          flowPath,
			DeviceTarget:      "device-ref:android-pixel-7",
			AppArtifactDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
}

func writeMobileScriptedJourney(t *testing.T, dir, flowPath string) string {
	t.Helper()
	flow := filepath.Join(dir, filepath.FromSlash(flowPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	pack := mobileScriptedJourneyPack(flowPath)
	body, err := json.Marshal(pack)
	require.NoError(t, err)
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "mobile-scripted-smoke.yaml"), body, 0o644))
	return pack.ID
}

func writeReadyMobileReadiness(t *testing.T, dir string) {
	t.Helper()
	writeMobileReadinessConfig(t, dir, `
device_inventory:
  devices:
    - device_ref: device-ref:android-pixel-7
      platform: android
simulator_emulator:
  targets:
    - target_ref: emulator-ref:android-34
      platform: android
app_artifact:
  path: .autopus/qa/mobile/apps/app.apk
  digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
credentials:
  refs:
    - credential-ref:android-dev
`)
}

func writeMissingAppMobileReadiness(t *testing.T, dir string) {
	t.Helper()
	writeMobileReadinessConfig(t, dir, `
device_inventory:
  devices:
    - device_ref: device-ref:android-pixel-7
      platform: android
simulator_emulator:
  targets:
    - target_ref: emulator-ref:android-34
      platform: android
credentials:
  refs:
    - credential-ref:android-dev
`)
}

func TestExecuteMobilePackResolvesHandleAndKeepsItOutOfManifest(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	runDir := filepath.Join(dir, "runs", "qa-1")
	fake := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}, flowExit: 0}
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")

	result, manifestPath, _ := executeMobilePack(Options{ProjectDir: dir, Lane: "mobile-scripted", deviceRunner: fake}, pack, filepath.Join(runDir, "_raw"), runDir)

	require.Equal(t, "emulator-5554", fake.LastHandle)
	require.Equal(t, "passed", result.Status)
	require.NotEmpty(t, manifestPath)
	manifest, err := qaevidence.LoadManifest(manifestPath)
	require.NoError(t, err)
	require.NotNil(t, manifest.SourceRefs.Mobile)
	assert.Equal(t, "device-ref:android-pixel-7", manifest.SourceRefs.Mobile.DeviceRef)
	raw, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "emulator-5554")
}

func TestExecuteMobilePackStatusFollowsFlowExit(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	runDir := filepath.Join(dir, "runs", "qa-pass")
	pass := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}, flowExit: 0}
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")

	passResult, passManifest, _ := executeMobilePack(Options{ProjectDir: dir, Lane: "mobile-scripted", deviceRunner: pass}, pack, filepath.Join(runDir, "_raw"), runDir)
	require.Equal(t, "passed", passResult.Status)
	require.Equal(t, 1, pass.FlowCalls)
	passManifestDoc, err := qaevidence.LoadManifest(passManifest)
	require.NoError(t, err)
	assert.Equal(t, "passed", passManifestDoc.Status)

	failDir := filepath.Join(dir, "runs", "qa-fail")
	fail := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}, flowExit: 1}
	failResult, failManifest, _ := executeMobilePack(Options{ProjectDir: dir, Lane: "mobile-scripted", deviceRunner: fail}, pack, filepath.Join(failDir, "_raw"), failDir)
	assert.Equal(t, "failed", failResult.Status)
	failManifestDoc, err := qaevidence.LoadManifest(failManifest)
	require.NoError(t, err)
	assert.Equal(t, "failed", failManifestDoc.Status)
	assert.NotEmpty(t, failManifestDoc.OracleResults.Checks[0].FailureSummary)
}

func TestExecuteMobileScriptedRejectsAIMaestroPackBeforeAnyFlow(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	flow := filepath.Join(dir, ".autopus", "qa", "mobile", "flows", "smoke.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")
	pack.PassFailAuthority = "ai"
	body, err := json.Marshal(pack)
	require.NoError(t, err)
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "mobile-ai.yaml"), body, 0o644))

	fake := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}}
	_, err = Execute(Options{ProjectDir: dir, Lane: "mobile-scripted", Output: filepath.Join(dir, "runs"), deviceRunner: fake})

	require.Error(t, err)
	assert.Equal(t, 0, fake.FlowCalls)
}
