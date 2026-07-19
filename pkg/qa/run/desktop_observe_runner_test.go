package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

func TestDesktopObservationRunner_SelectedProviderOnlyAndExactOperationOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		selected desktopobserve.RuntimeProvider
	}{
		{name: "local", selected: desktopobserve.RuntimeProviderLocal},
		{name: "orca", selected: desktopobserve.RuntimeProviderOrca},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
			orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
			runner := newDesktopObservationRunner(local, orca)
			outcome, err := runner.Run(context.Background(), desktopRunRequest(test.selected))
			require.NoError(t, err)
			assert.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
			selected := local
			alternate := orca
			if test.selected == desktopobserve.RuntimeProviderOrca {
				selected, alternate = orca, local
			}
			assert.Equal(t, []string{"handshake", "capabilities", "permissions", "list_apps", "list_windows", "get_state"}, selected.calls)
			assert.Empty(t, alternate.calls)
			assert.Equal(t, desktopobserve.ReadOnlyOperations(), capabilityNames(outcome.RuntimeReceipt))
		})
	}
}

func TestDesktopProviderProtocolMismatch_BlocksBeforeOperationsWithoutFallback(t *testing.T) {
	t.Parallel()
	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	local.protocolVersion = desktopobserve.ProtocolVersion + 1
	orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	runner := newDesktopObservationRunner(local, orca)
	outcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonProviderProtocolMismatch, *outcome.ReasonCode)
	assert.NotEqual(t, desktopobserve.VerdictPassed, outcome.Verdict)
	assert.Nil(t, outcome.SemanticProjection)
	assert.Equal(t, []string{"handshake"}, local.calls)
	assert.Empty(t, orca.calls)
}

func TestDesktopObservationRunner_UnavailableSelectedProviderDoesNotFallback(t *testing.T) {
	t.Parallel()
	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	local.handshakeErr = errors.New("private provider path failed")
	orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	runner := newDesktopObservationRunner(local, orca)
	outcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonProviderUnavailable, *outcome.ReasonCode)
	assert.Nil(t, outcome.SemanticProjection)
	assert.Equal(t, []string{"handshake"}, local.calls)
	assert.Empty(t, orca.calls)
	body, marshalErr := json.Marshal(outcome.RuntimeReceipt)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(body), "private provider path failed")
}

func TestDesktopObservationRunner_AccessibilityDeniedStopsBeforeTargetCalls(t *testing.T) {
	t.Parallel()
	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	local.accessibilityGranted = false
	orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	outcome, err := newDesktopObservationRunner(local, orca).Run(
		context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal),
	)
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonAccessibilityPermissionMissing, *outcome.ReasonCode)
	assert.Nil(t, outcome.SemanticProjection)
	assert.Equal(t, []string{"handshake", "capabilities", "permissions"}, local.calls)
	assert.Empty(t, orca.calls)
	assert.Equal(t, desktopobserve.RedactionNotRequired, outcome.RuntimeReceipt.Redaction.Status)
	require.NotNil(t, outcome.RuntimeReceipt.NextStep)
	assert.NotEmpty(t, *outcome.RuntimeReceipt.NextStep)
}
func TestDesktopObservationRunner_MissingOrInvalidProviderCallsNeitherClient(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider desktopobserve.RuntimeProvider
		want     error
	}{
		{name: "missing", provider: "", want: desktopobserve.ErrRuntimeProviderRequired},
		{name: "invalid", provider: "automatic", want: desktopobserve.ErrRuntimeProviderInvalid},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
			orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
			runner := newDesktopObservationRunner(local, orca)
			request := desktopRunRequest(test.provider)

			_, err := runner.Run(context.Background(), request)
			require.Error(t, err)
			assert.ErrorIs(t, err, test.want)
			assert.Empty(t, local.calls)
			assert.Empty(t, orca.calls)
		})
	}
}

func TestDesktopObservationRunner_RequiresExactlyOneExactAppAndWindow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		configure func(*fakeDesktopClient)
		reason    desktopobserve.ReasonCode
		calls     []string
	}{
		{name: "zero apps", configure: func(client *fakeDesktopClient) { client.apps = []desktopobserve.AppSummary{} }, reason: desktopobserve.ReasonTargetAppNotFound, calls: []string{"handshake", "capabilities", "permissions", "list_apps"}},
		{name: "one wrong app", configure: func(client *fakeDesktopClient) { client.apps = []desktopobserve.AppSummary{{AppRef: "other-app"}} }, reason: desktopobserve.ReasonTargetAppNotFound, calls: []string{"handshake", "capabilities", "permissions", "list_apps"}},
		{name: "duplicate app", configure: func(client *fakeDesktopClient) {
			client.apps = []desktopobserve.AppSummary{{AppRef: "autopus-desktop"}, {AppRef: "autopus-desktop"}}
		}, reason: desktopobserve.ReasonProviderProtocolMismatch, calls: []string{"handshake", "capabilities", "permissions", "list_apps"}},
		{name: "extra app", configure: func(client *fakeDesktopClient) {
			client.apps = []desktopobserve.AppSummary{{AppRef: "autopus-desktop"}, {AppRef: "other-app"}}
		}, reason: desktopobserve.ReasonProviderProtocolMismatch, calls: []string{"handshake", "capabilities", "permissions", "list_apps"}},
		{name: "zero windows", configure: func(client *fakeDesktopClient) { client.windows = []desktopobserve.WindowSummary{} }, reason: desktopobserve.ReasonTargetWindowNotFound, calls: []string{"handshake", "capabilities", "permissions", "list_apps", "list_windows"}},
		{name: "one wrong window", configure: func(client *fakeDesktopClient) {
			client.windows = []desktopobserve.WindowSummary{{WindowRef: "other-window"}}
		}, reason: desktopobserve.ReasonTargetWindowNotFound, calls: []string{"handshake", "capabilities", "permissions", "list_apps", "list_windows"}},
		{name: "duplicate window", configure: func(client *fakeDesktopClient) {
			client.windows = []desktopobserve.WindowSummary{{WindowRef: "main-window"}, {WindowRef: "main-window"}}
		}, reason: desktopobserve.ReasonProviderProtocolMismatch, calls: []string{"handshake", "capabilities", "permissions", "list_apps", "list_windows"}},
		{name: "extra window", configure: func(client *fakeDesktopClient) {
			client.windows = []desktopobserve.WindowSummary{{WindowRef: "main-window"}, {WindowRef: "other-window"}}
		}, reason: desktopobserve.ReasonProviderProtocolMismatch, calls: []string{"handshake", "capabilities", "permissions", "list_apps", "list_windows"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
			alternate := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
			test.configure(local)
			outcome, err := newDesktopObservationRunner(local, alternate).Run(
				context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal),
			)
			require.NoError(t, err)
			require.NotNil(t, outcome.ReasonCode)
			assert.Equal(t, test.reason, *outcome.ReasonCode)
			assert.Nil(t, outcome.SemanticProjection)
			assert.Equal(t, test.calls, local.calls)
			assert.Empty(t, alternate.calls)
		})
	}
}

func TestLocalOrcaOracleParity_ProviderIdentityIsOnlyReceiptDifference(t *testing.T) {
	t.Parallel()
	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	runner := newDesktopObservationRunner(local, orca)
	localOutcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	orcaOutcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderOrca))
	require.NoError(t, err)
	assert.Equal(t, normalizedObservationBytes(t, localOutcome), normalizedObservationBytes(t, orcaOutcome))
	assert.NotEqual(t, localOutcome.RuntimeReceipt.Provider.Name, orcaOutcome.RuntimeReceipt.Provider.Name)
	require.NotNil(t, orcaOutcome.SemanticProjection)
	assert.Equal(t, []desktopobserve.Action{
		desktopobserve.ActionPress,
		desktopobserve.ActionShowMenu,
	}, orcaOutcome.SemanticProjection.Root.AdvertisedActions)
	assert.Zero(t, orca.rawIndexCalls)
	assert.Zero(t, orca.actionCalls)
	assert.Zero(t, orca.screenshotCalls)
}

func TestDesktopObservationExecutePack_BypassesGenericRawCommandPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	runDir := filepath.Join(dir, "run")
	rawRoot := filepath.Join(runDir, "_raw")
	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	runner := newDesktopObservationRunner(local, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca))
	pack := desktopObservationPack()
	result, manifestPath, checks := executePack(Options{
		ProjectDir:      dir,
		Lane:            "desktop-native",
		RuntimeProvider: desktopobserve.RuntimeProviderLocal,
		desktopRunner:   runner,
	}, pack, rawRoot, runDir)
	assert.Equal(t, "passed", result.Status)
	assert.NotEmpty(t, manifestPath)
	assert.NotEmpty(t, checks)
	assert.NoDirExists(t, rawRoot)
	require.NotNil(t, result.DesktopObservation)

	manifest, err := qaevidence.LoadManifest(manifestPath)
	require.NoError(t, err)
	require.NotNil(t, manifest.OracleResults.DesktopObservation)
	assert.Equal(t, qaevidence.Runner{Name: "desktop-accessibility-observe"}, manifest.Runner)
	assert.Equal(t, "desktop-accessibility-observe", manifest.ScenarioRef)
	assert.Equal(t, "desktop-accessibility-observe", manifest.SourceRefs.JourneyID)
	assert.Equal(t, "step-1", manifest.SourceRefs.StepID)
	assert.Empty(t, manifest.SourceRefs.OwnedPaths)
	assert.Empty(t, manifest.SourceRefs.DoNotModifyPaths)
	assert.Empty(t, manifest.SourceRefs.OracleThresholds)
	assert.Nil(t, manifest.SourceRefs.Mobile)
	assert.Empty(t, manifest.ReproductionCommand)
	assert.Empty(t, manifest.RepairPromptRef)
	require.Len(t, manifest.OracleResults.Checks, 1)
	manifestCheck := manifest.OracleResults.Checks[0]
	assert.Equal(t, desktopobserve.DeterministicCheckSemanticLandmarks, manifestCheck.ID)
	assert.Equal(t, "desktop_accessibility_semantic", manifestCheck.Type)
	assert.Equal(t, "passed", manifestCheck.Status)
	assert.Empty(t, manifestCheck.Expected)
	assert.Empty(t, manifestCheck.Actual)
	assert.Empty(t, manifestCheck.ArtifactRefs)
	assert.Empty(t, manifestCheck.FailureSummary)
	resultBody, err := json.Marshal(result.DesktopObservation)
	require.NoError(t, err)
	manifestBody, err := json.Marshal(manifest.OracleResults.DesktopObservation)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(resultBody, manifestBody))
	require.Len(t, manifest.Artifacts, 1)
	artifactBody, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	artifact, err := desktopobserve.DecodeObservationEvidence(artifactBody)
	require.NoError(t, err)
	inlineBody, err := json.Marshal(manifest.OracleResults.DesktopObservation)
	require.NoError(t, err)
	decodedArtifactBody, err := json.Marshal(artifact)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(inlineBody, decodedArtifactBody))
}

func TestDesktopObservationExecutePack_BlockedManifestUsesTypedFailureProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	local.accessibilityGranted = false
	pack := desktopObservationPack()
	result, manifestPath, checks := executePack(Options{
		ProjectDir: dir, Lane: "desktop-native", RuntimeProvider: desktopobserve.RuntimeProviderLocal,
		desktopRunner: newDesktopObservationRunner(
			local, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
		),
	}, pack, filepath.Join(dir, "run", "_raw"), filepath.Join(dir, "run"))
	assert.Equal(t, "blocked", result.Status)
	assert.Equal(t, string(desktopobserve.ReasonAccessibilityPermissionMissing), result.FailureSummary)
	require.NotNil(t, result.DesktopObservation)
	require.NotEmpty(t, manifestPath)
	require.Len(t, checks, 1)
	manifest, err := qaevidence.LoadManifest(manifestPath)
	require.NoError(t, err)
	require.Len(t, manifest.OracleResults.Checks, 1)
	assert.Equal(t, "blocked", manifest.OracleResults.Checks[0].Status)
	assert.Equal(t, "desktop observation blocked", manifest.OracleResults.Checks[0].FailureSummary)
	assert.Empty(t, manifest.OracleResults.Checks[0].Expected)
	assert.Empty(t, manifest.OracleResults.Checks[0].Actual)
	assert.Empty(t, manifest.OracleResults.Checks[0].ArtifactRefs)
}
