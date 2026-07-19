package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopObservationRelease_ReleaseLanesExactBytesAndOrderUnchanged(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(ReleaseLanes())
	require.NoError(t, err)
	assert.Equal(t,
		`["fast","browser-staging","desktop-native","gui-explore","mobile-readiness","canary-explicit","evidence-dashboard"]`,
		string(body),
	)
}

func TestDesktopObservationRelease_CommandFreeNativePackIsExecutable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDesktopObservationReleaseJourney(t, dir)
	plan, err := BuildPlan(Options{
		ProjectDir:      dir,
		Profile:         "prelaunch",
		DryRun:          true,
		RuntimeProvider: desktopobserve.RuntimeProviderLocal,
	})
	require.NoError(t, err)

	row := findJourneyPackRow(t, plan.JourneyPacks, "desktop-native")
	assert.Equal(t, "desktop-accessibility-observe", row.Adapter)
	assert.False(t, row.CommandDeclared)
	assert.True(t, row.Executable)
	assertNoDesktopNativeGap(t, plan.SetupGaps)
}

func TestDesktopObservationRelease_RuntimeProviderMapsIntoRunOptions(t *testing.T) {
	t.Parallel()

	got := qarunOptionsForLane(Options{
		ProjectDir:      "desktop",
		Profile:         "prelaunch",
		RunOutputRoot:   "runs",
		RuntimeProvider: desktopobserve.RuntimeProviderOrca,
	}, "desktop-native")
	assert.Equal(t, desktopobserve.RuntimeProviderOrca, got.RuntimeProvider)
	assert.Equal(t, "desktop-native", got.Lane)
}

func TestDesktopObservationRelease_NonPassStatesNeverPromote(t *testing.T) {
	t.Parallel()

	rows := []LaneRow{
		{Lane: "desktop-native", LanePolicy: LanePolicyMust, Status: LaneStatusFailed, Severity: SeverityHigh},
		{Lane: "desktop-native", LanePolicy: LanePolicyMust, Status: LaneStatusBlocked, Severity: SeverityHigh},
		{Lane: "desktop-native", LanePolicy: LanePolicyMust, Status: LaneStatusSetupGap, SetupGapClass: SetupGapMissingJourneyPack, Severity: SeverityHigh},
		{Lane: "desktop-native", LanePolicy: LanePolicyMust, Status: LaneStatus("quarantined"), Severity: SeverityHigh},
	}
	for _, row := range rows {
		normalized := NormalizeLaneRow(row)
		assert.NotEqual(t, LaneVerdictPass, normalized.LaneVerdict, row.Status)
		assert.NotEqual(t, GateStatusPassed, AggregateGateStatus([]LaneRow{normalized}), row.Status)
	}
}

func TestDesktopObservationRelease_PassedRunWithBlockedRedactionFailsClosed(t *testing.T) {
	t.Parallel()

	row := runResultLaneRow(
		ProfilePolicy{MustLanes: []string{"desktop-native"}},
		"desktop-native",
		LaneRunResult{Status: LaneStatusPassed, RedactionStatus: RedactionBlocked},
		nil,
	)
	assert.Equal(t, LaneStatusBlocked, row.Status)
	normalized := NormalizeLaneRow(row)
	assert.Equal(t, LaneVerdictBlock, normalized.LaneVerdict)
	assert.Equal(t, GateStatusBlocked, AggregateGateStatus([]LaneRow{normalized}))
}

func writeDesktopObservationReleaseJourney(t *testing.T, dir string) {
	t.Helper()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := `id: desktop-accessibility-observe
title: Desktop accessibility observation
surface: desktop
lanes: [desktop-native]
adapter: {id: desktop-accessibility-observe}
command: {}
checks:
  - {id: semantic-landmarks, type: desktop_accessibility_semantic}
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
  acceptance_refs: [AC-QAMESH12-016]
pass_fail_authority: deterministic
`
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "desktop-accessibility-observe.yaml"), []byte(body), 0o600))
}

func assertNoDesktopNativeGap(t *testing.T, gaps []SetupGapRow) {
	t.Helper()
	for _, gap := range gaps {
		assert.NotEqual(t, "desktop-native", gap.Lane, gap)
	}
}
