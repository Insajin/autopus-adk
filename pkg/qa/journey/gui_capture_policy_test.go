package journey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
)

func TestGUICapturePolicy_ParsesFromJourneyYAML(t *testing.T) {
	t.Parallel()

	pack := loadGUICapturePack(t, `
capture:
  mode: always
  streams: [screenshot, console, network, trace]
  screenshot: per-step
  console_severity: warning
  retain_local: true
  replay_script: required
`)
	policy := pack.GUI.Capture
	assert.True(t, policy.Declared())
	assert.True(t, policy.Enabled())
	assert.Equal(t, capture.ModeAlways, policy.Mode)
	assert.Equal(t, []string{"screenshot", "console", "network", "trace"}, policy.Streams)
	assert.Equal(t, capture.ScreenshotPerStep, policy.Screenshot)
	assert.Equal(t, capture.SeverityWarning, policy.ConsoleSeverity)
	assert.True(t, policy.RetainLocal)
	assert.Equal(t, capture.ReplayRequired, policy.ReplayScript)
	assert.Empty(t, policy.Unknown)
	require.NoError(t, Validate(pack, t.TempDir()))
}

// TestGUICapturePolicy_RejectsMisspelledKey is the regression this contract
// exists for: a typo used to disable evidence collection silently.
func TestGUICapturePolicy_RejectsMisspelledKey(t *testing.T) {
	t.Parallel()

	pack := loadGUICapturePack(t, `
capture:
  mode: always
  streams: [console]
  screenshots: per-step
`)
	assert.NotEmpty(t, pack.GUI.Capture.Unknown)
	err := Validate(pack, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown fields")
	assert.Contains(t, err.Error(), "screenshots")
}

// TestGUICapturePolicy_RawPublicationStaysRejected pins the invariant capture
// depends on: raw media may never be published, so a capture policy retaining
// local media can never be paired with publish_raw.
func TestGUICapturePolicy_RawPublicationStaysRejected(t *testing.T) {
	t.Parallel()

	pack := loadGUICapturePack(t, `
capture:
  mode: always
  streams: [trace]
  retain_local: true
`)
	pack.GUI.ArtifactRetention.PublishRaw = true
	err := Validate(pack, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "may not publish raw screenshots, traces, or videos")
}

// TestGUICapturePolicy_AllowsBlockedNetworkWithNetworkStream inverts an earlier
// rule. While `blocked` was a label with no runtime effect, pairing it with the
// network stream looked contradictory. Now the mode actually aborts xhr/fetch, so
// the network evidence is the list of requests the policy stopped — which is
// exactly what proves the UI's empty and error states.
func TestGUICapturePolicy_AllowsBlockedNetworkWithNetworkStream(t *testing.T) {
	t.Parallel()

	pack := loadGUICapturePack(t, `
capture:
  mode: always
  streams: [network]
`)
	pack.GUI.NetworkPolicy.Mode = "blocked"
	require.NoError(t, Validate(pack, t.TempDir()))
}

// TestGUICapturePolicy_RejectedOnNonGUIAdapter prevents a pack from promising
// capture evidence that no runtime produces.
func TestGUICapturePolicy_RejectedOnNonGUIAdapter(t *testing.T) {
	t.Parallel()

	pack := loadGUICapturePack(t, `
capture:
  mode: always
  streams: [console]
`)
	pack.Adapter.ID = "go-test"
	pack.Command = Command{Argv: []string{"go", "test", "./..."}}
	err := Validate(pack, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported by the gui-explore adapter")
}

func TestGUICapturePolicy_EmptyBlockStaysValid(t *testing.T) {
	t.Parallel()

	pack := loadGUICapturePack(t, "")
	assert.False(t, pack.GUI.Capture.Declared())
	require.NoError(t, Validate(pack, t.TempDir()))
}

// loadGUICapturePack builds a minimal valid gui-explore pack and splices the
// given `gui.capture` YAML fragment in, so each test exercises real decoding
// rather than a hand-built struct.
func loadGUICapturePack(t *testing.T, captureFragment string) Pack {
	t.Helper()
	body := `
id: gui-capture
title: GUI capture
surface: frontend
lanes: [gui-explore]
adapter:
  id: gui-explore
command:
  argv: ["npm", "exec", "playwright", "test"]
  cwd: .
  timeout: 60s
checks:
  - id: gui-capture
    type: gui_exploration
gui:
  allowed_origins:
    - http://127.0.0.1:4173
  forbidden_actions:
    - mutation
  selector_strategy: role-first
  network_policy:
    mode: local-only
  artifact_retention:
    publish_raw: false
` + indentFragment(captureFragment)
	var pack Pack
	require.NoError(t, yaml.Unmarshal([]byte(body), &pack))
	return pack
}

func indentFragment(fragment string) string {
	if fragment == "" {
		return ""
	}
	out := ""
	for _, line := range splitLines(fragment) {
		if line == "" {
			continue
		}
		out += "  " + line + "\n"
	}
	return out
}

func splitLines(value string) []string {
	var lines []string
	current := ""
	for _, char := range value {
		if char == '\n' {
			lines = append(lines, current)
			current = ""
			continue
		}
		current += string(char)
	}
	return append(lines, current)
}
