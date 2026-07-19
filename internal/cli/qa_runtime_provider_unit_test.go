package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQARuntimeProvider_RequirementDetectionUsesSelectedExecutionScope(t *testing.T) {
	t.Parallel()

	dir := writeRuntimeProviderUnitJourney(t)
	assert.True(t, projectRequiresQARuntimeProvider(dir))
	assert.False(t, projectRequiresQARuntimeProvider(t.TempDir()))

	tests := []struct {
		name string
		opts qaRunOptions
		want bool
	}{
		{name: "explicit observation adapter", opts: qaRunOptions{AdapterID: qaDesktopObservationAdapterID}, want: true},
		{name: "explicit generic adapter", opts: qaRunOptions{AdapterID: "custom-command"}},
		{name: "unfiltered project", opts: qaRunOptions{ProjectDir: dir}, want: true},
		{name: "matching journey", opts: qaRunOptions{ProjectDir: dir, JourneyID: qaDesktopObservationAdapterID}, want: true},
		{name: "other journey", opts: qaRunOptions{ProjectDir: dir, JourneyID: "other"}},
		{name: "matching lane", opts: qaRunOptions{ProjectDir: dir, Lane: "desktop-native"}, want: true},
		{name: "other lane", opts: qaRunOptions{ProjectDir: dir, Lane: "fast"}},
		{name: "no observation pack", opts: qaRunOptions{ProjectDir: t.TempDir()}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, runRequiresQARuntimeProvider(test.opts))
		})
	}
}

func TestQARuntimeProvider_InvalidJourneyLoadDoesNotMaskDownstreamValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "invalid.yaml"), []byte("id:\tinvalid\n"), 0o600))

	assert.False(t, projectRequiresQARuntimeProvider(dir))
	assert.False(t, runRequiresQARuntimeProvider(qaRunOptions{ProjectDir: dir}))
}

func TestQARuntimeProvider_FullSingleChildValidatesBeforeBootstrap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	child := filepath.Join(root, "desktop")
	require.NoError(t, os.MkdirAll(filepath.Join(child, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(child, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(child, "src-tauri", "Cargo.toml"), []byte("[package]\nname='fixture'\n"), 0o600))
	writeRuntimeProviderUnitJourneyAt(t, child)
	cmd := newQAFullCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runQAFull(cmd, qaFullOptions{ProjectDir: root, Bootstrap: true, Format: "json"})

	require.Error(t, err)
	payload := decodeJSONMap(t, output.Bytes())
	assert.Equal(t, "qa_runtime_provider_required", payload["error"].(map[string]any)["code"])
	assert.NoFileExists(t, filepath.Join(child, ".autopus", "qa", "domain-readiness", "catalog.json"))
}

func writeRuntimeProviderUnitJourney(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeRuntimeProviderUnitJourneyAt(t, dir)
	return dir
}

func writeRuntimeProviderUnitJourneyAt(t *testing.T, dir string) {
	t.Helper()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := []byte(`id: desktop-accessibility-observe
title: Desktop accessibility observation
surface: desktop
lanes: [desktop-native]
adapter: {id: desktop-accessibility-observe}
command: {}
checks: [{id: semantic-landmarks, type: desktop_accessibility_semantic}]
artifacts: []
desktop_observation:
  platform: macos
  operations: [capabilities, permissions, list_apps, list_windows, get_state]
  app_ref: autopus-desktop
  window_ref: main-window
  required_landmarks:
    - {role: AXApplication, name: Autopus, required_state: enabled}
    - {role: AXWindow, name: Autopus, required_state: focused}
source_refs:
  source_spec: SPEC-QAMESH-012
  acceptance_refs: [AC-QAMESH12-012]
pass_fail_authority: deterministic
`)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "desktop-accessibility-observe.yaml"), body, 0o600))
}
