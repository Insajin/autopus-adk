package run

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestOrcaDesktopClient_ExactFiveReadOnlyCommandsNormalizePublicEvidence(t *testing.T) {
	client, executor := newHermeticOrcaClient(t)
	runner := newDesktopObservationRunner(nil, client)
	request := desktopRunRequest(desktopobserve.RuntimeProviderOrca)
	request.Policy.AllowedNames = append(request.Policy.AllowedNames, "Disclosure")

	outcome, err := runner.Run(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
	require.NotNil(t, outcome.SemanticProjection)
	assert.Equal(t, "orca-computer-use-macos", outcome.RuntimeReceipt.Provider.Name)
	assert.Equal(t, []string{
		"computer capabilities --json",
		"computer permissions --json",
		"computer list-apps --json",
		"computer list-windows --app co.autopus.desktop --json",
		"computer get-app-state --app co.autopus.desktop --no-screenshot --json",
	}, executor.recordedCalls())

	projection := outcome.SemanticProjection
	require.Len(t, projection.Root.Children, 1)
	require.Len(t, projection.Root.Children[0].Children, 1)
	disclosure := projection.Root.Children[0].Children[0]
	assert.Equal(t, desktopobserve.Role("AXButton"), disclosure.Role)
	assert.Equal(t, "Disclosure", disclosure.Name)
	assert.Equal(t, []desktopobserve.Action{desktopobserve.ActionPress}, disclosure.AdvertisedActions)
	require.NotNil(t, disclosure.SemanticState.Enabled)
	require.NotNil(t, disclosure.SemanticState.Expanded)
	assert.True(t, *disclosure.SemanticState.Enabled)
	assert.False(t, *disclosure.SemanticState.Expanded)

	public, err := json.Marshal(outcome)
	require.NoError(t, err)
	for _, forbidden := range []string{
		"treeText", "snapshot", "elementCount", "focusedElementId", "windowId",
		"pid", "/private/", "Autopus fixture status", "Ready", "zoom the window",
	} {
		assert.NotContains(t, string(public), forbidden)
	}
}

func TestOrcaDesktopClient_CanonicalDigestMatchesLocalForSameFixture(t *testing.T) {
	client, _ := newHermeticOrcaClient(t)
	runner := newDesktopObservationRunner(nil, client)
	request := desktopRunRequest(desktopobserve.RuntimeProviderOrca)
	request.Policy.AllowedNames = append(request.Policy.AllowedNames, "Disclosure")
	orca, err := runner.Run(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, orca.SemanticProjection)

	local, err := desktopobserve.NormalizeProjection(
		localOrcaParityFixture(),
		func(value string) (string, error) { return value, nil },
	)
	require.NoError(t, err)
	assert.Equal(t, local.Digest, orca.SemanticProjection.Digest)
	assert.Equal(t, local.CanonicalJSON, orca.SemanticProjection.CanonicalJSON)
	assert.Equal(t, local.SchemaVersion, orca.SemanticProjection.SchemaVersion)
	assert.Equal(t, local.AppRef, orca.SemanticProjection.AppRef)
	assert.Equal(t, local.WindowRef, orca.SemanticProjection.WindowRef)
}

func TestOrcaDesktopClient_FreshCSPRNGStateAndNodeRefs(t *testing.T) {
	client, _ := newHermeticOrcaClient(t)
	ctx := context.Background()
	_, err := client.Handshake(ctx)
	require.NoError(t, err)
	_, err = client.Permissions(ctx)
	require.NoError(t, err)
	_, err = client.ListApps(ctx)
	require.NoError(t, err)
	_, err = client.ListWindows(ctx, "autopus-desktop")
	require.NoError(t, err)

	first, err := client.GetState(ctx, "autopus-desktop", "main-window")
	require.NoError(t, err)
	second, err := client.GetState(ctx, "autopus-desktop", "main-window")
	require.NoError(t, err)
	assert.NotEqual(t, first.StateRef, second.StateRef)
	assert.Regexp(t, `^state-[0-9a-f]{64}$`, first.StateRef)
	assert.Regexp(t, `^state-[0-9a-f]{64}$`, second.StateRef)
	firstRefs, secondRefs := collectOrcaNodeRefs(first.Root), collectOrcaNodeRefs(second.Root)
	require.Len(t, firstRefs, 3)
	require.Len(t, secondRefs, 3)
	for _, ref := range append(firstRefs, secondRefs...) {
		assert.Regexp(t, `^node-[0-9a-f]{64}$`, ref)
	}
	for _, firstRef := range firstRefs {
		assert.NotContains(t, secondRefs, firstRef)
	}
}

func TestOrcaDesktopClient_ExpandedStateChangesAndBaselineRecoversDigest(t *testing.T) {
	run := func(expanded bool) desktopobserve.SemanticProjection {
		executor := &fakeOrcaCommandExecutor{responses: orcaTestResponses(expanded)}
		client, err := newOrcaDesktopClientWith("/private/test/orca", executor, &countingOrcaReader{})
		require.NoError(t, err)
		runner := newDesktopObservationRunner(nil, client)
		request := desktopRunRequest(desktopobserve.RuntimeProviderOrca)
		request.Policy.AllowedNames = append(request.Policy.AllowedNames, "Disclosure")
		outcome, err := runner.Run(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, outcome.SemanticProjection)
		return *outcome.SemanticProjection
	}
	baseline := run(false)
	expanded := run(true)
	recovered := run(false)
	assert.NotEqual(t, baseline.Digest, expanded.Digest)
	assert.Equal(t, baseline.Digest, recovered.Digest)
	assert.Equal(t, baseline.CanonicalJSON, recovered.CanonicalJSON)
}

func TestOrcaDesktopClient_DeniedAccessibilityStopsBeforeTargetEnumeration(t *testing.T) {
	client, executor := newHermeticOrcaClient(t)
	executor.responses["computer permissions --json"] = orcaPermissionsFixture("denied")
	runner := newDesktopObservationRunner(nil, client)
	request := desktopRunRequest(desktopobserve.RuntimeProviderOrca)
	request.Policy.AllowedNames = append(request.Policy.AllowedNames, "Disclosure")
	outcome, err := runner.Run(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonAccessibilityPermissionMissing, *outcome.ReasonCode)
	assert.Equal(t, []string{
		"computer capabilities --json", "computer permissions --json",
	}, executor.recordedCalls())
	body, err := json.Marshal(outcome)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "/private/provider/helper.app")
}

func localOrcaParityFixture() desktopobserve.SemanticProjection {
	enabled, focused, expanded := true, true, false
	return desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   "provider-local", AppRef: "autopus-desktop",
		WindowRef: "main-window", StateRef: "state-local-parity",
		Root: desktopobserve.SemanticNode{
			Role: desktopobserve.RoleApplication, Name: "Autopus",
			SemanticState: desktopobserve.SemanticState{Enabled: &enabled},
			Children: []desktopobserve.SemanticNode{{
				Role: desktopobserve.RoleWindow, Name: "Autopus",
				SemanticState: desktopobserve.SemanticState{Focused: &focused},
				Children: []desktopobserve.SemanticNode{{
					Role: desktopobserve.Role("AXButton"), Name: "Disclosure",
					SemanticState:     desktopobserve.SemanticState{Enabled: &enabled, Expanded: &expanded},
					AdvertisedActions: []desktopobserve.Action{desktopobserve.ActionPress},
				}},
			}},
		},
	}
}

func collectOrcaNodeRefs(root desktopobserve.SemanticNode) []string {
	refs := []string{root.NodeRef}
	for _, child := range root.Children {
		refs = append(refs, collectOrcaNodeRefs(child)...)
	}
	return refs
}
