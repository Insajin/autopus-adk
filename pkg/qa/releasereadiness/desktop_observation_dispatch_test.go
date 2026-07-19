package releasereadiness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestDesktopObservationDispatch_CommandFreePackPropagatesRuntimeProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	desktopSignals(t, root)
	pack := readinessDesktopObservationPack()
	var captured qarun.Options
	row := dispatchLane(Options{
		ProjectDir: root, Approve: true, RuntimeProvider: desktopobserve.RuntimeProviderOrca,
	}, pack, []string{"desktop"}, func(options qarun.Options) (qarun.Result, error) {
		captured = options
		return successfulReadinessDesktopObservationResult(t, options.RuntimeProvider), nil
	})

	assert.Equal(t, string(release.LaneStatusPassed), row.Status)
	assert.Equal(t, "desktop-native", captured.Lane)
	assert.Equal(t, "desktop-accessibility-observe", captured.AdapterID)
	assert.Equal(t, desktopobserve.RuntimeProviderOrca, captured.RuntimeProvider)
}

func TestDesktopObservationDispatch_ConfiguredPackIsAdditiveAndDispatchedOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	desktopSignals(t, root)
	pack := readinessDesktopObservationPack()
	writePack(t, root, pack)
	adapters := []string{}
	providers := []desktopobserve.RuntimeProvider{}

	payload, err := orchestrateWith(Options{
		ProjectDir: root, Approve: true, RuntimeProvider: desktopobserve.RuntimeProviderLocal,
	}, func(options qarun.Options) (qarun.Result, error) {
		adapters = append(adapters, options.AdapterID)
		providers = append(providers, options.RuntimeProvider)
		return successfulReadinessDesktopObservationResult(t, options.RuntimeProvider), nil
	})
	require.NoError(t, err)
	assert.Equal(t, string(PhaseExecuted), payload.Phase)
	assert.Equal(t, 1, countString(adapters, "desktop-accessibility-observe"))
	for index, adapterID := range adapters {
		if adapterID == "desktop-accessibility-observe" {
			assert.Equal(t, desktopobserve.RuntimeProviderLocal, providers[index])
		}
	}
}

func TestDesktopObservationDispatch_NonPassStatusesNeverPromote(t *testing.T) {
	t.Parallel()

	statuses := []string{
		string(release.LaneStatusFailed),
		string(release.LaneStatusBlocked),
		string(release.LaneStatusSetupGap),
		"quarantined",
	}
	for _, status := range statuses {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			verdict := aggregateVerdict([]LaneRow{{
				Lane: "desktop-native", Status: status, DeterministicAuthority: true,
			}})
			assert.NotEqual(t, string(release.GateStatusPassed), verdict.Status)
			assert.True(t, verdict.DeterministicAuthority)
		})
	}
}

func TestDesktopObservationDispatch_PassedOverallCannotLaunderObservationFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result qarun.Result
	}{
		{name: "bare passed result", result: qarun.Result{Status: "passed"}},
		{name: "adapter blocked", result: qarun.Result{
			Status: "passed", AdapterResults: []qarun.AdapterResult{{
				Adapter: "desktop-accessibility-observe", Status: "blocked",
			}},
		}},
		{name: "adapter setup gap", result: qarun.Result{
			Status: "passed", AdapterResults: []qarun.AdapterResult{{
				Adapter: "desktop-accessibility-observe", Status: "setup_gap",
				SetupGap: &qarun.SetupGap{Adapter: "desktop-accessibility-observe", Reason: "provider unavailable"},
			}},
		}},
		{name: "deterministic check blocked", result: qarun.Result{
			Status: "passed", Checks: []qarun.IndexCheck{{
				ID: "desktop-semantic-landmarks", Adapter: "desktop-accessibility-observe", Status: "blocked",
			}},
		}},
		{name: "redaction blocked", result: qarun.Result{
			Status: "passed", RedactionStatus: qarun.RedactionStatus{Status: "blocked"},
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			row := laneRowFromRun(LaneRow{
				Lane: "desktop-native", DeterministicAuthority: true,
			}, test.result, nil)
			assert.Equal(t, string(release.LaneStatusBlocked), row.Status)
			verdict := aggregateVerdict([]LaneRow{row})
			assert.Equal(t, string(release.GateStatusBlocked), verdict.Status)
		})
	}
}

func successfulReadinessDesktopObservationResult(t *testing.T, provider desktopobserve.RuntimeProvider) qarun.Result {
	t.Helper()
	enabled, focused := true, true
	providerName, providerRef := "autopus-desktop-local", "provider-local"
	if provider == desktopobserve.RuntimeProviderOrca {
		providerName, providerRef = "orca-computer-use-macos", "provider-orca"
	}
	projection, err := desktopobserve.NormalizeProjection(desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   providerRef,
		AppRef:        "autopus-desktop",
		WindowRef:     "main-window",
		StateRef:      "state-readiness",
		Root: desktopobserve.SemanticNode{
			Role:          desktopobserve.RoleApplication,
			Name:          "Autopus",
			SemanticState: desktopobserve.SemanticState{Enabled: &enabled},
			Children: []desktopobserve.SemanticNode{{
				Role:          desktopobserve.RoleWindow,
				Name:          "Autopus",
				SemanticState: desktopobserve.SemanticState{Focused: &focused},
			}},
		},
	}, func(value string) (string, error) { return value, nil })
	require.NoError(t, err)
	capabilities := make([]desktopobserve.CapabilityStatus, 0, len(desktopobserve.ReadOnlyOperations()))
	for _, operation := range desktopobserve.ReadOnlyOperations() {
		capabilities = append(capabilities, desktopobserve.CapabilityStatus{
			Name: operation, Status: desktopobserve.CapabilitySupported,
		})
	}
	observation := desktopobserve.ObservationEvidence{
		SemanticProjection: &projection,
		DeterministicChecks: []desktopobserve.DeterministicCheck{{
			ID: "desktop-semantic-landmarks", Status: desktopobserve.CheckPassed,
		}},
		RuntimeReceipt: desktopobserve.RuntimeReceipt{
			SchemaVersion: desktopobserve.RuntimeReceiptSchemaVersion,
			Provider: desktopobserve.ProviderIdentity{
				Name: providerName, Version: "1.0.0", ProtocolVersion: desktopobserve.ProtocolVersion,
			},
			Scope:             desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeWindow, PublicRef: "main-window"},
			CapabilitySummary: capabilities,
			Redaction:         desktopobserve.RedactionReceipt{Status: desktopobserve.RedactionApplied},
			Quarantine:        desktopobserve.QuarantineReceipt{Status: desktopobserve.QuarantineCleared},
		},
	}
	require.NoError(t, observation.RuntimeReceipt.Validate())
	return qarun.Result{
		Status: "passed",
		AdapterResults: []qarun.AdapterResult{{
			Adapter: "desktop-accessibility-observe", JourneyID: "desktop-accessibility-observe", Status: "passed",
			DesktopObservation: &observation,
		}},
		Checks: []qarun.IndexCheck{{
			ID: "desktop-semantic-landmarks", JourneyID: "desktop-accessibility-observe",
			Adapter: "desktop-accessibility-observe", Status: "passed",
		}},
		RedactionStatus: qarun.RedactionStatus{Status: "passed"},
	}
}

func readinessDesktopObservationPack() journey.Pack {
	return journey.Pack{
		ID: "desktop-accessibility-observe", Title: "Desktop accessibility observation",
		Surface: "desktop", Lanes: []string{"desktop-native"},
		Adapter: journey.AdapterRef{ID: "desktop-accessibility-observe"},
		Checks:  []journey.Check{{ID: "semantic-landmarks", Type: "desktop_accessibility_semantic"}},
		DesktopObservation: journey.DesktopObservationPolicy{
			Platform: "macos", Operations: []string{"capabilities", "permissions", "list_apps", "list_windows", "get_state"},
			AppRef: "autopus-desktop", WindowRef: "main-window",
			RequiredLandmarks: []journey.DesktopObservationLandmark{
				{Role: "AXApplication", Name: "Autopus", RequiredState: "enabled"},
				{Role: "AXWindow", Name: "Autopus", RequiredState: "focused"},
			},
		},
		SourceRefs: journey.SourceRefs{
			SourceSpec: "SPEC-QAMESH-012", AcceptanceRefs: []string{"AC-QAMESH12-016"},
		},
		PassFailAuthority: "deterministic",
	}
}

func countString(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
