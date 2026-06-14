package run

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPlanMobileScriptedSelectsMaestroWhenReady(t *testing.T) {
	dir := t.TempDir()
	writeReadyMobileReadiness(t, dir)
	journeyID := writeMobileScriptedJourney(t, dir, ".autopus/qa/mobile/flows/smoke.yaml")

	scripted, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-scripted", Output: filepath.Join(dir, "runs")})
	require.NoError(t, err)
	require.NotNil(t, scripted.MobileReadiness)
	assert.Equal(t, "ready", scripted.MobileReadiness.Status)
	assert.Equal(t, []string{"maestro-scripted"}, scripted.SelectedAdapters)
	assert.Equal(t, []string{journeyID}, scripted.SelectedJourneys)

	readiness, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-readiness", Output: filepath.Join(dir, "runs")})
	require.NoError(t, err)
	assert.Empty(t, readiness.SelectedAdapters)
}

func TestBuildPlanMobileScriptedBlocksWhenAppArtifactMissing(t *testing.T) {
	dir := t.TempDir()
	writeMissingAppMobileReadiness(t, dir)
	writeMobileScriptedJourney(t, dir, ".autopus/qa/mobile/flows/smoke.yaml")

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-scripted", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	require.NotNil(t, plan.MobileReadiness)
	assert.NotEqual(t, "ready", plan.MobileReadiness.Status)
	assert.Empty(t, plan.SelectedAdapters)
	assert.Empty(t, plan.SelectedJourneys)
	assert.Empty(t, plan.ManifestOutputPreviewPaths)
	assert.Contains(t, setupGapReasons(plan.SetupGaps), "missing_app_artifact: app artifact digest and safe project-relative path are required")
}

func TestMobileScriptedNotReadyDoesNotResolveDevices(t *testing.T) {
	dir := t.TempDir()
	writeMissingAppMobileReadiness(t, dir)
	writeMobileScriptedJourney(t, dir, ".autopus/qa/mobile/flows/smoke.yaml")

	plan, err := BuildPlan(Options{ProjectDir: dir, Lane: "mobile-scripted", Output: filepath.Join(dir, "runs")})
	require.NoError(t, err)
	assert.Empty(t, plan.SelectedAdapters)

	fake := &fakeMobileDeviceRunner{handles: map[string]string{"device-ref:android-pixel-7": "emulator-5554"}}
	_, _ = Execute(Options{ProjectDir: dir, Lane: "mobile-scripted", Output: filepath.Join(dir, "runs"), deviceRunner: fake})
	assert.Equal(t, 0, fake.ResolveCalls)
}
