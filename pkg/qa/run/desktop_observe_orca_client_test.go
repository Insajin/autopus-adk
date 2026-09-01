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

	outcome, err := runner.Run(context.Background(), orcaFixtureRunRequest())
	require.NoError(t, err)
	require.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
	require.NotNil(t, outcome.SemanticProjection)
	assert.Equal(t, "orca-computer-use-macos", outcome.RuntimeReceipt.Provider.Name)
	// REQ-3: every provider request addresses the pack's provider_app_id. The
	// operation set is unchanged and still read-only.
	assert.Equal(t, []string{
		"computer capabilities --json",
		"computer permissions --json",
		"computer list-apps --json",
		"computer list-windows --app co.autopus.desktop --json",
		"computer get-app-state --app co.autopus.desktop --no-screenshot --json",
	}, executor.recordedCalls())

	// The projection is the two declared landmarks and nothing else. It used to be
	// a hardcoded three-node tree ending in an AXButton "Disclosure" that the
	// decoder invented rather than observed; the real 7-node tree contains no such
	// element, and the pack declares no landmark below the window, so publishing
	// one would be publishing an assumption.
	projection := outcome.SemanticProjection
	assert.Equal(t, desktopobserve.RoleApplication, projection.Root.Role)
	assert.Equal(t, orcaFixtureAppName, projection.Root.Name)
	require.NotNil(t, projection.Root.SemanticState.Enabled)
	assert.True(t, *projection.Root.SemanticState.Enabled)
	require.Len(t, projection.Root.Children, 1)
	window := projection.Root.Children[0]
	assert.Equal(t, desktopobserve.RoleWindow, window.Role)
	assert.Equal(t, orcaFixtureWindowTitle, window.Name)
	require.NotNil(t, window.SemanticState.Focused)
	assert.True(t, *window.SemanticState.Focused)
	// The pack's third landmark, published because the pack declared it. The tree
	// carries six other nodes below the window and none of them appear.
	require.Len(t, window.Children, 1)
	button := window.Children[0]
	assert.Equal(t, desktopobserve.Role("AXButton"), button.Role)
	assert.Equal(t, orcaFixtureButtonName, button.Name)
	require.NotNil(t, button.SemanticState.Enabled)
	assert.True(t, *button.SemanticState.Enabled, "the renderer inlined no (disabled) marker")
	assert.Empty(t, button.Children)
	// No advertised actions anywhere: the tree's action metadata is raw AppKit
	// data, and the builder publishes declared state only.
	assert.Empty(t, projection.Root.AdvertisedActions)
	assert.Empty(t, window.AdvertisedActions)
	assert.Empty(t, button.AdvertisedActions)

	public, err := json.Marshal(outcome)
	require.NoError(t, err)
	// The last four entries are observed content from the real capture: two
	// localized strings the app displays, the raw AppKit action metadata on the
	// full-screen button, and a localized role phrase.
	for _, forbidden := range []string{
		"treeText", "snapshot", "elementCount", "focusedElementId", "windowId",
		"pid", "/private/", orcaFixtureDocumentTitle, orcaFixtureStatusText,
		"zoom the window", "standard window",
	} {
		assert.NotContains(t, string(public), forbidden)
	}
}

// AC-QAMESH13-010: a projection accepted from one provider is accepted from the
// other, so the two must canonicalize identically for the same observation.
func TestOrcaDesktopClient_CanonicalDigestMatchesLocalForSameFixture(t *testing.T) {
	client, _ := newHermeticOrcaClient(t)
	runner := newDesktopObservationRunner(nil, client)
	orca, err := runner.Run(context.Background(), orcaFixtureRunRequest())
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
	// One ref per published node, so this count is the pack's landmark count and
	// never the observed app's node count.
	require.Len(t, firstRefs, len(orcaFixtureLandmarks()))
	require.Len(t, secondRefs, len(orcaFixtureLandmarks()))
	for _, ref := range append(firstRefs, secondRefs...) {
		assert.Regexp(t, `^node-[0-9a-f]{64}$`, ref)
	}
	for _, firstRef := range firstRefs {
		assert.NotContains(t, secondRefs, firstRef)
	}
}

// Digest stability: a change in a published dimension changes the digest, and
// returning to the original observation recovers it byte for byte.
//
// The dimension is a state marker on a pack-declared node, which is what the
// deleted fixture path encoded too - except it read the marker out of a 14-line
// UI nobody ships, and the node it published was invented rather than observed.
// Here the marker is parsed out of the rendered tree and the node is published
// only because the pack declared its name.
//
// "(disabled)" is the marker worth pinning: the renderer marks only the negative
// case, so enabled is derived from the marker's ABSENCE. A mapping that read it
// positively would report every control as disabled and nothing else here would
// notice.
//
// It asserts on the client rather than the runner because a disabled control
// legitimately fails the pack's enabled landmark, and the verdict is not what is
// under test.
func TestOrcaDesktopClient_DeclaredNodeStateChangesAndBaselineRecoversDigest(t *testing.T) {
	observe := func(fixture orcaTreeFixture) desktopobserve.SemanticProjection {
		client, _ := newTargetedOrcaClient(t, fixture)
		normalized, err := desktopobserve.NormalizeProjection(
			orcaObserveOnce(t, client), func(value string) (string, error) { return value, nil },
		)
		require.NoError(t, err)
		return normalized
	}
	button := func(projection desktopobserve.SemanticProjection) desktopobserve.SemanticNode {
		return projection.Root.Children[0].Children[0]
	}

	baseline := observe(orcaTreeFixture{})
	disabled := observe(orcaTreeFixture{ButtonState: "disabled"})
	recovered := observe(orcaTreeFixture{})

	require.NotNil(t, button(baseline).SemanticState.Enabled)
	require.NotNil(t, button(disabled).SemanticState.Enabled)
	assert.True(t, *button(baseline).SemanticState.Enabled)
	assert.False(t, *button(disabled).SemanticState.Enabled)
	assert.NotEqual(t, baseline.Digest, disabled.Digest)
	assert.Equal(t, baseline.Digest, recovered.Digest)
	assert.Equal(t, baseline.CanonicalJSON, recovered.CanonicalJSON)
}

// Window focus is the other published state the tree reports, through the
// trailing focus line rather than through a node marker. A focused element id the
// rendered body does not contain is legal wire shape - Orca reports focus for the
// session - and must project as an unfocused window rather than erroring.
func TestOrcaDesktopClient_FocusOutsideTheRenderedTreeProjectsUnfocused(t *testing.T) {
	client, _ := newTargetedOrcaClient(t, orcaTreeFixture{FocusedID: 91})
	projection := orcaObserveOnce(t, client)
	window := projection.Root.Children[0]
	require.NotNil(t, window.SemanticState.Focused)
	assert.False(t, *window.SemanticState.Focused)
}

func TestOrcaDesktopClient_DeniedAccessibilityStopsBeforeTargetEnumeration(t *testing.T) {
	client, executor := newHermeticOrcaClient(t)
	executor.responses["computer permissions --json"] = orcaPermissionsFixture("denied")
	runner := newDesktopObservationRunner(nil, client)
	outcome, err := runner.Run(context.Background(), orcaFixtureRunRequest())
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

// localOrcaParityFixture is the projection a local provider would publish for the
// same observation, hand-written so the parity assertion has an independent
// reference rather than comparing the builder against itself.
//
// It mirrors the builder's contract exactly: one node per declared landmark, each
// carrying ONLY the state its landmark asked about, and no advertised actions.
// The previous fixture also had three nodes, but its third was an AXButton
// "Disclosure" carrying Enabled, Expanded and an AXPress action - a shape that
// matched the deleted hardcoded decoder and nothing a provider observes.
func localOrcaParityFixture() desktopobserve.SemanticProjection {
	enabled, focused, buttonEnabled := true, true, true
	return desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   "provider-local", AppRef: "autopus-desktop",
		WindowRef: "main-window", StateRef: "state-local-parity",
		Root: desktopobserve.SemanticNode{
			Role: desktopobserve.RoleApplication, Name: orcaFixtureAppName,
			SemanticState: desktopobserve.SemanticState{Enabled: &enabled},
			Children: []desktopobserve.SemanticNode{{
				Role: desktopobserve.RoleWindow, Name: orcaFixtureWindowTitle,
				SemanticState: desktopobserve.SemanticState{Focused: &focused},
				Children: []desktopobserve.SemanticNode{{
					Role: desktopobserve.Role("AXButton"), Name: orcaFixtureButtonName,
					SemanticState: desktopobserve.SemanticState{Enabled: &buttonEnabled},
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
