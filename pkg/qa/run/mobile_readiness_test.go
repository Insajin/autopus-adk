package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPlanMobileReadinessDoesNotUseDetectedFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-readiness", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	require.NotNil(t, plan.MobileReadiness)
	assert.Equal(t, "setup_gap", plan.MobileReadiness.Status)
	assert.Contains(t, plan.DetectedAdapters, "go-test")
	assert.Empty(t, plan.SelectedAdapters)
	assert.Empty(t, plan.SelectedJourneys)
	assert.Empty(t, plan.ManifestOutputPreviewPaths)
	assert.Contains(t, setupGapReasons(plan.SetupGaps), "missing_device_inventory: device inventory is required before mobile execution")
}

func TestBuildPlanMobileReadinessBlocksUnsafePlanningOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeMobileReadinessConfig(t, dir, `
app_artifact:
  path: /Users/alice/private/app.ipa
credentials:
  refs:
    - auth_cookie=session-secret
`)

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-readiness", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	require.NotNil(t, plan.MobileReadiness)
	assert.Equal(t, "blocked", plan.MobileReadiness.RedactionStatus.Status)
	assert.NotContains(t, plan.MobileReadiness.AppArtifact.Path, "/Users/alice")
	assert.NotContains(t, plan.MobileReadiness.Credentials.Refs, "session-secret")
}

func TestBuildPlanMobileReadinessUsesPublicRelativePaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	output := filepath.Join(dir, "runs")

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-readiness", Output: output})

	require.NoError(t, err)
	assert.Equal(t, "runs", plan.OutputRoot)
	assert.NotContains(t, plan.RunIndexPreviewPath, dir)
	assert.Equal(t, ".autopus/qa/journeys", plan.HarnessContract.JourneyPackRoot)
}

func TestExecuteMobileReadinessBlocksConfiguredJourneyWhenSetupGap(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "maestro-ran")
	installFakeMobileBinary(t, "maestro", "printf run > "+shellQuote(marker))
	writeMobileJourney(t, dir, ".autopus/qa/mobile/flows/login.yaml")

	result, err := Execute(Options{ProjectDir: dir, Lane: "mobile-readiness", Output: filepath.Join(dir, "runs")})

	require.Error(t, err)
	assert.Equal(t, "blocked", result.Status)
	assert.Empty(t, result.ManifestPaths)
	assert.NoFileExists(t, marker)
	assert.Contains(t, setupGapReasons(result.SetupGaps), "missing_device_inventory: device inventory is required before mobile execution")
}

func TestExecuteMobileReadinessWritesNormalizedMobileEvidence(t *testing.T) {
	dir := t.TempDir()
	installFakeMobileBinary(t, "maestro", "echo ok")
	writeMobileReadinessConfig(t, dir, `
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
	writeMobileJourney(t, dir, ".autopus/qa/mobile/flows/login.yaml")

	result, err := Execute(Options{ProjectDir: dir, Lane: "mobile-readiness", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	require.Len(t, result.ManifestPaths, 1)
	manifest, err := qaevidence.LoadManifest(result.ManifestPaths[0])
	require.NoError(t, err)
	assert.Equal(t, "mobile", manifest.Surface)
	require.NotNil(t, manifest.SourceRefs.Mobile)
	assert.Equal(t, "device-ref:ios-sim", manifest.SourceRefs.Mobile.DeviceRef)
	for _, artifact := range manifest.Artifacts {
		assert.Equal(t, "sanitized_log", artifact.Kind)
	}
}

func TestBuildManifestMobilePublicationRejectsRawDeviceName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := mobileCommandResult(t, dir)
	pack := mobileJourneyPack(".autopus/qa/mobile/flows/login.yaml")
	pack.Mobile.DeviceTarget = "Alice iPhone 15"
	manifest := buildManifest(Options{ProjectDir: dir, Lane: "mobile-readiness"}, pack, result, []IndexCheck{{
		ID:        "mobile-check",
		JourneyID: pack.ID,
		Adapter:   pack.Adapter.ID,
		Status:    "passed",
		Expected:  "exit_code=0",
		Actual:    "exit_code=0",
	}})

	_, err := qaevidence.WriteFinalManifest(manifest, filepath.Join(dir, "final"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "device_ref")
}

func writeMobileReadinessConfig(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, ".autopus", "qa", "mobile", "readiness.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func setupGapReasons(gaps []SetupGap) []string {
	reasons := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		reasons = append(reasons, gap.Reason)
	}
	return reasons
}

func writeMobileJourney(t *testing.T, dir, flowPath string) {
	t.Helper()
	flow := filepath.Join(dir, filepath.FromSlash(flowPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	body, err := json.Marshal(mobileJourneyPack(flowPath))
	require.NoError(t, err)
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "mobile-login.yaml"), body, 0o644))
}

func mobileJourneyPack(flowPath string) journey.Pack {
	return journey.Pack{
		ID:      "mobile-login",
		Surface: "mobile",
		Lanes:   []string{"mobile-readiness"},
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
		Command: journey.Command{Argv: []string{"maestro", "test", flowPath}, CWD: ".", Timeout: "60s"},
		Checks:  []journey.Check{{ID: "login-visible", Type: "mobile_check"}},
		Mobile: journey.MobilePolicy{
			FlowPath:          flowPath,
			DeviceTarget:      "device-ref:ios-sim",
			AppArtifactDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
}

func installFakeMobileBinary(t *testing.T, name, body string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	path := filepath.Join(bin, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

func mobileCommandResult(t *testing.T, dir string) commandResult {
	t.Helper()
	raw := filepath.Join(dir, "raw")
	require.NoError(t, os.MkdirAll(raw, 0o755))
	stdout := filepath.Join(raw, "stdout.log")
	stderr := filepath.Join(raw, "stderr.log")
	require.NoError(t, os.WriteFile(stdout, []byte("ok\n"), 0o644))
	require.NoError(t, os.WriteFile(stderr, []byte(""), 0o644))
	now := time.Now().UTC()
	return commandResult{
		Status:     "passed",
		StdoutPath: stdout,
		StderrPath: stderr,
		StartedAt:  now,
		EndedAt:    now.Add(time.Second),
		DurationMS: 1000,
		Command:    "maestro test .autopus/qa/mobile/flows/login.yaml",
	}
}
