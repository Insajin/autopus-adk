package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryContainsRequiredAdapters(t *testing.T) {
	t.Parallel()

	ids := map[string]bool{}
	for _, item := range Registry() {
		ids[item.ID] = true
		assert.NotEmpty(t, item.Surfaces)
		assert.NotEmpty(t, item.DefaultLanes)
		assert.NotEmpty(t, item.ArtifactCapabilities)
	}
	for _, id := range []string{"go-test", "node-script", "vitest", "jest", "playwright", "gui-explore", "design-visual", "maestro-scripted", "appium-mobile-explore", "pytest", "cargo-test", "auto-test-run", "auto-verify", "canary-template", "custom-command"} {
		assert.True(t, ids[id], id)
	}
	gui, ok := ByID("gui-explore")
	require.True(t, ok)
	assert.Contains(t, gui.ArtifactCapabilities, "journey_graph")
	assert.Contains(t, gui.ArtifactCapabilities, "screenshot_quarantine_ref")
	designVisual, ok := ByID("design-visual")
	require.True(t, ok)
	assert.Equal(t, []string{"design-visual"}, designVisual.DefaultLanes)
	assert.Contains(t, designVisual.ArtifactCapabilities, "design_pack")
	assert.Contains(t, designVisual.ArtifactCapabilities, "visual_gate_report")
	assert.Contains(t, designVisual.ArtifactCapabilities, "figma_node_metadata")
	assert.Contains(t, designVisual.SetupGapReasonCodes, "screenshot_baseline_missing")
	assert.Contains(t, designVisual.SetupGapReasonCodes, "figma_token_missing")
	maestro, ok := ByID("maestro-scripted")
	require.True(t, ok)
	assert.Equal(t, []string{"mobile-readiness"}, maestro.DefaultLanes)
	assert.Contains(t, maestro.SupportedPlatforms, "ios")
	assert.Contains(t, maestro.ReadinessFields, "app_artifact")
	assert.NotEmpty(t, maestro.SetupGapReason)
	assert.Contains(t, maestro.SetupGapReasonCodes, "missing_device_inventory")
}

func TestDetectFindsProjectAdapters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js"},"devDependencies":{"jest":"latest"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pytest.ini"), []byte("[pytest]\n"), 0o644))

	var ids []string
	for _, item := range Detect(dir) {
		ids = append(ids, item.AdapterID)
	}
	assert.Contains(t, ids, "go-test")
	assert.Contains(t, ids, "node-script")
	assert.Contains(t, ids, "jest")
	assert.Contains(t, ids, "pytest")
}

func TestDetectFindsPytestFromPyprojectAndPlaywright(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.pytest.ini_options]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"@playwright/test":"latest","vitest":"latest"}}`), 0o644))

	var ids []string
	for _, item := range Detect(dir) {
		ids = append(ids, item.AdapterID)
	}
	assert.Contains(t, ids, "pytest")
	assert.Contains(t, ids, "playwright")
	assert.Contains(t, ids, "vitest")
}

func TestWithSetupGapsAndUnknownAdapter(t *testing.T) {
	t.Parallel()

	items := WithSetupGaps()
	require.NotEmpty(t, items)
	_, ok := ByID("missing")
	assert.False(t, ok)
	canary, ok := ByID("canary-template")
	require.True(t, ok)
	assert.Equal(t, "canary-template", canary.ID)
}
