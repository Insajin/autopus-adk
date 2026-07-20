package journey

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDesktopObservationPolicy_CommandFreeNativePackIsValid(t *testing.T) {
	t.Parallel()

	pack := validDesktopObservationPack()
	require.NoError(t, Validate(pack, t.TempDir()))
	assert.Empty(t, pack.Command)
	assert.Empty(t, pack.Artifacts)
	assert.Equal(t, []string{
		"capabilities",
		"permissions",
		"list_apps",
		"list_windows",
		"get_state",
	}, pack.DesktopObservation.Operations)
	assert.Equal(t, "autopus-desktop", pack.DesktopObservation.AppRef)
	assert.Equal(t, "main-window", pack.DesktopObservation.WindowRef)
	assert.Equal(t, []DesktopObservationLandmark{
		{Role: "AXApplication", Name: "Autopus", RequiredState: "enabled"},
		{Role: "AXWindow", Name: "Autopus", RequiredState: "focused"},
	}, pack.DesktopObservation.RequiredLandmarks)

	body, err := yaml.Marshal(pack)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "runtime_provider")
}

func TestDesktopObservationPolicy_UnsafeCommandArtifactOrOperationFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Pack)
	}{
		{name: "shell run", mutate: func(pack *Pack) { pack.Command.Run = "sh -c whoami" }},
		{name: "argv", mutate: func(pack *Pack) { pack.Command.Argv = []string{"auto", "qa", "run"} }},
		{name: "file cwd", mutate: func(pack *Pack) { pack.Command.CWD = "." }},
		{name: "timeout", mutate: func(pack *Pack) { pack.Command.Timeout = "10s" }},
		{name: "environment", mutate: func(pack *Pack) { pack.Command.EnvAllowlist = []string{"PATH"} }},
		{name: "raw artifact", mutate: func(pack *Pack) { pack.Artifacts = []Artifact{{Kind: "raw_ax", Path: "raw.json"}} }},
		{name: "safe artifact still forbidden", mutate: func(pack *Pack) { pack.Artifacts = []Artifact{{Kind: "summary", Path: "summary.json"}} }},
		{name: "click", mutate: func(pack *Pack) { pack.DesktopObservation.Operations[4] = "click" }},
		{name: "screenshot", mutate: func(pack *Pack) { pack.DesktopObservation.Operations[4] = "screenshot" }},
		{name: "raw tree", mutate: func(pack *Pack) { pack.DesktopObservation.Operations[4] = "raw_tree" }},
		{name: "shell", mutate: func(pack *Pack) { pack.DesktopObservation.Operations[4] = "shell" }},
		{name: "file", mutate: func(pack *Pack) { pack.DesktopObservation.Operations[4] = "file" }},
		{name: "missing operation", mutate: func(pack *Pack) { pack.DesktopObservation.Operations = pack.DesktopObservation.Operations[:4] }},
		{name: "extra operation", mutate: func(pack *Pack) {
			pack.DesktopObservation.Operations = append(pack.DesktopObservation.Operations, "permissions")
		}},
		{name: "duplicate operation", mutate: func(pack *Pack) { pack.DesktopObservation.Operations[4] = "permissions" }},
		{name: "reordered operation", mutate: func(pack *Pack) {
			pack.DesktopObservation.Operations[0], pack.DesktopObservation.Operations[1] = pack.DesktopObservation.Operations[1], pack.DesktopObservation.Operations[0]
		}},
		{name: "missing app alias", mutate: func(pack *Pack) { pack.DesktopObservation.AppRef = "" }},
		{name: "noncanonical app alias", mutate: func(pack *Pack) { pack.DesktopObservation.AppRef = "com.autopus.desktop" }},
		{name: "missing window alias", mutate: func(pack *Pack) { pack.DesktopObservation.WindowRef = "" }},
		{name: "noncanonical window alias", mutate: func(pack *Pack) { pack.DesktopObservation.WindowRef = "window-1" }},
		{name: "missing landmarks", mutate: func(pack *Pack) { pack.DesktopObservation.RequiredLandmarks = nil }},
		{name: "incomplete landmarks", mutate: func(pack *Pack) { pack.DesktopObservation.RequiredLandmarks[1].RequiredState = "" }},
		{name: "duplicate landmark", mutate: func(pack *Pack) {
			pack.DesktopObservation.RequiredLandmarks[1] = pack.DesktopObservation.RequiredLandmarks[0]
		}},
		{name: "noncanonical landmark role", mutate: func(pack *Pack) { pack.DesktopObservation.RequiredLandmarks[0].Role = "application" }},
		{name: "noncanonical landmark name", mutate: func(pack *Pack) { pack.DesktopObservation.RequiredLandmarks[0].Name = "Other" }},
		{name: "noncanonical landmark state", mutate: func(pack *Pack) { pack.DesktopObservation.RequiredLandmarks[0].RequiredState = "focused" }},
		{name: "wrong lane", mutate: func(pack *Pack) { pack.Lanes = []string{"fast"} }},
		{name: "wrong surface", mutate: func(pack *Pack) { pack.Surface = "frontend" }},
		{name: "ai authority", mutate: func(pack *Pack) { pack.PassFailAuthority = "ai" }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pack := validDesktopObservationPack()
			test.mutate(&pack)
			err := Validate(pack, t.TempDir())
			require.Error(t, err)
			var validationErr *ValidationError
			require.True(t, errors.As(err, &validationErr))
			assert.Equal(t, "qa_journey_desktop_observation_policy_invalid", validationErr.Code)
		})
	}
}

func TestDesktopObservationPolicy_GoldenYAMLLoadsThroughValidator(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-observe.yaml")
	raw := strings.Replace(validDesktopObservationYAML(), "__EXTRA__", "", 1)
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))
	pack, err := LoadFile(path)
	require.NoError(t, err)
	require.NoError(t, Validate(pack, dir))
	assert.Empty(t, pack.Artifacts)
	assert.Equal(t, validDesktopObservationPack().DesktopObservation, pack.DesktopObservation)
}

func TestDesktopObservationPolicy_UnknownMutationAndRuntimeYAMLFieldsFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra string
	}{
		{name: "click", extra: "  click: true\n"},
		{name: "screenshot", extra: "  screenshot: true\n"},
		{name: "raw tree", extra: "  raw_tree: true\n"},
		{name: "shell", extra: "  shell: whoami\n"},
		{name: "file", extra: "  file: /tmp/raw.json\n"},
		{name: "runtime provider", extra: "runtime_provider: local\n"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "desktop-observe.yaml")
			raw := strings.Replace(validDesktopObservationYAML(), "__EXTRA__", test.extra, 1)
			require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))
			_, err := LoadFile(path)
			require.Error(t, err)
		})
	}
}

func validDesktopObservationPack() Pack {
	return Pack{
		ID:      "desktop-accessibility-observe",
		Title:   "Observe signed Autopus Desktop accessibility state",
		Surface: "desktop",
		Lanes:   []string{"desktop-native"},
		Adapter: AdapterRef{ID: "desktop-accessibility-observe"},
		Checks: []Check{{
			ID:   "desktop-semantic-landmarks",
			Type: "desktop_accessibility_semantic",
		}},
		DesktopObservation: DesktopObservationPolicy{
			Platform:   "macos",
			Operations: []string{"capabilities", "permissions", "list_apps", "list_windows", "get_state"},
			AppRef:     "autopus-desktop",
			WindowRef:  "main-window",
			RequiredLandmarks: []DesktopObservationLandmark{
				{Role: "AXApplication", Name: "Autopus", RequiredState: "enabled"},
				{Role: "AXWindow", Name: "Autopus", RequiredState: "focused"},
			},
		},
		SourceRefs: SourceRefs{
			SourceSpec:     "SPEC-QAMESH-012",
			AcceptanceRefs: []string{"AC-QAMESH12-001", "AC-QAMESH12-017"},
		},
		PassFailAuthority: "deterministic",
	}
}

func validDesktopObservationYAML() string {
	return `id: desktop-accessibility-observe
title: Observe signed Autopus Desktop accessibility state
surface: desktop
lanes: [desktop-native]
adapter:
  id: desktop-accessibility-observe
command: {}
checks:
  - id: desktop-semantic-landmarks
    type: desktop_accessibility_semantic
artifacts: []
desktop_observation:
  platform: macos
  operations: [capabilities, permissions, list_apps, list_windows, get_state]
  app_ref: autopus-desktop
  window_ref: main-window
  required_landmarks:
    - role: AXApplication
      name: Autopus
      required_state: enabled
    - role: AXWindow
      name: Autopus
      required_state: focused
__EXTRA__source_refs:
  source_spec: SPEC-QAMESH-012
  acceptance_refs: [AC-QAMESH12-001, AC-QAMESH12-017]
pass_fail_authority: deterministic
`
}
