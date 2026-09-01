package run

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

// SPEC-QAMESH-013 REQ-3: the provider must answer about the app the pack named.
//
// Before the pack supplied that identity, these cases could not exist: the
// decoder compared against a compiled-in bundle id, so "the provider answered
// about a different app" and "the harness asked about the wrong app" were the
// same state. Each case here drives one identity apart from the rest and expects
// a refusal, because a projection built from a mismatched observation would be
// published under the pack's refs while describing something else.
func TestOrcaStateDecoder_ProviderIdentityDriftFailsClosed(t *testing.T) {
	t.Parallel()

	binding, target := orcaStateTestBinding(), orcaStateTestTarget()
	tests := []struct {
		name    string
		fixture orcaTreeFixture
		mutate  func(map[string]any)
	}{
		// The envelope names another app. Caught by validOrcaSnapshotApp before
		// the tree is parsed at all.
		{name: "envelope bundle id drift", mutate: func(value map[string]any) {
			snapshot(value)["app"].(map[string]any)["bundleId"] = "com.apple.finder"
		}},
		{name: "envelope app name drift", mutate: func(value map[string]any) {
			snapshot(value)["app"].(map[string]any)["name"] = "Finder"
		}},
		// The envelope agrees but the rendered tree's own header names another
		// app. This is the case a compiled-in constant could never catch, since
		// the constant and the fixture were written to match each other.
		{name: "tree header bundle id drift", fixture: orcaTreeFixture{AppID: "com.apple.finder"}},
		// A different pid than the bound window: the provider answered about
		// another process, or the app restarted between list-windows and
		// get-app-state.
		{name: "tree header pid drift", fixture: orcaTreeFixture{PID: orcaTestPID + 1}},
		{name: "envelope window title drift", mutate: func(value map[string]any) {
			snapshot(value)["window"].(map[string]any)["title"] = "Finder"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			raw := orcaStateFixture(test.fixture)
			if test.mutate != nil {
				raw = mutateOrcaJSON(raw, test.mutate)
			}
			projection, err := decodeOrcaState(
				raw, orcaTestRuntimeID, binding, &countingOrcaReader{}, target,
			)
			require.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
			assert.Empty(t, projection)
		})
	}
}

// An unset target must refuse rather than match. A target arrives from the pack
// through the runner; if that injection is ever skipped, an empty expected
// identity that compared equal to an empty observed one would restore exactly
// the behaviour this SPEC removed, only without a constant to grep for.
func TestOrcaStateDecoder_UnsetTargetFailsClosed(t *testing.T) {
	t.Parallel()

	binding := orcaStateTestBinding()
	full := orcaStateTestTarget()
	targets := map[string]desktopProviderTarget{
		"entirely unset": {},
		"no provider app id": {
			AppName: full.AppName, WindowTitle: full.WindowTitle,
			AppRef: full.AppRef, WindowRef: full.WindowRef,
		},
		"no app name": {
			ProviderAppID: full.ProviderAppID, WindowTitle: full.WindowTitle,
			AppRef: full.AppRef, WindowRef: full.WindowRef,
		},
		"no window title": {
			ProviderAppID: full.ProviderAppID, AppName: full.AppName,
			AppRef: full.AppRef, WindowRef: full.WindowRef,
		},
	}
	for name, target := range targets {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			projection, err := decodeOrcaState(
				orcaStateFixture(orcaTreeFixture{}), orcaTestRuntimeID,
				binding, &countingOrcaReader{}, target,
			)
			require.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
			assert.Empty(t, projection)
		})
	}
}

// The same rule one layer earlier: enumeration cannot match an app or a window
// against an identity nobody supplied.
func TestOrcaTargets_UnsetExpectedIdentityFailsClosed(t *testing.T) {
	t.Parallel()

	_, _, err := decodeOrcaApps(orcaAppsFixture(1), orcaTestRuntimeID, "", orcaFixtureAppName)
	assert.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
	_, _, err = decodeOrcaApps(orcaAppsFixture(1), orcaTestRuntimeID, orcaFixtureAppID, "")
	assert.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)

	for _, identity := range [][3]string{
		{"", orcaFixtureAppName, orcaFixtureWindowTitle},
		{orcaFixtureAppID, "", orcaFixtureWindowTitle},
		{orcaFixtureAppID, orcaFixtureAppName, ""},
	} {
		_, _, err = decodeOrcaWindows(
			orcaWindowsFixture(1), orcaTestRuntimeID, orcaTestPID,
			identity[0], identity[1], identity[2],
		)
		assert.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
	}
}

// A pack that names an app which is running under a different bundle id finds no
// app rather than the wrong one. Zero matches is a target-not-found signal, not
// an envelope fault, so the operator is told the alias missed.
func TestOrcaTargets_ForeignBundleIDMatchesNothing(t *testing.T) {
	t.Parallel()

	pid, matches, err := decodeOrcaApps(
		orcaAppsFixture(1), orcaTestRuntimeID, "com.apple.finder", "Finder",
	)
	require.NoError(t, err)
	assert.Zero(t, matches, "the fixture runs co.autopus.desktop and com.example.other")
	assert.Zero(t, pid)
}

// REQ-5 and REQ-7: a declared landmark the observed tree does not carry reports
// itself by name.
//
// This is the case the previous taxonomy folded into provider unavailability. In
// each case the tree is well formed and the envelope agrees with what the pack
// declared; only the tree's own header differs, so the refusal has to name the
// landmark rather than blame the provider's lifecycle or the wire format.
//
// The variation goes through the REQUEST, not the target, because the declared
// landmarks now come from Policy.MinimumLandmarks: mutating a target field alone
// would leave the landmark list still naming the captured value and prove nothing.
func TestOrcaStateDecoder_DeclaredLandmarkMismatchReportsItsOwnReason(t *testing.T) {
	t.Parallel()

	binding := orcaStateTestBinding()
	tests := []struct {
		name     string
		observed orcaTreeFixture
		declare  func(*DesktopObservationRunRequest)
		wantRole desktopobserve.Role
		wantName string
	}{
		{
			name:     "window title differs from the declared landmark",
			observed: orcaTreeFixture{WindowTitle: "Autopus Desktop — Settings"},
			declare:  func(*DesktopObservationRunRequest) {},
			wantRole: desktopobserve.RoleWindow,
			wantName: orcaFixtureWindowTitle,
		},
		{
			name:     "app name differs from the declared landmark",
			observed: orcaTreeFixture{AppName: "Autopus"},
			declare:  func(*DesktopObservationRunRequest) {},
			wantRole: desktopobserve.RoleApplication,
			wantName: orcaFixtureAppName,
		},
		{
			// A suffix of the real title must not satisfy the declaration. The
			// header carries the label verbatim, so equality is the honest rule;
			// suffix matching exists only for body nodes, where the renderer
			// prefixes localized role words.
			name: "declared title is a suffix of the observed one",
			declare: func(request *DesktopObservationRunRequest) {
				request.WindowTitle = "Desktop"
				request.Policy.MinimumLandmarks[1].Name = "Desktop"
			},
			wantRole: desktopobserve.RoleWindow,
			wantName: "Desktop",
		},
		{
			// A declared node below the window that the tree does not contain is
			// the same class of failure as a missing window, and must report the
			// role and name the pack declared rather than the app's own labels.
			name: "declared deeper landmark is absent from the tree",
			declare: func(request *DesktopObservationRunRequest) {
				request.Policy.MinimumLandmarks[2].Name = "Cancel run"
			},
			wantRole: desktopobserve.Role("AXButton"),
			wantName: "Cancel run",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := orcaFixtureRunRequest()
			test.declare(&request)
			target := targetFromRequest(request)
			// The envelope has to agree with the declaration, or the envelope
			// check refuses first and this proves nothing about landmarks.
			raw := mutateOrcaJSON(orcaStateFixture(test.observed), func(value map[string]any) {
				snapshot(value)["window"].(map[string]any)["title"] = target.WindowTitle
				snapshot(value)["app"].(map[string]any)["name"] = target.AppName
			})
			projection, err := decodeOrcaState(
				raw, orcaTestRuntimeID, binding, &countingOrcaReader{}, target,
			)
			require.Error(t, err)
			assert.Empty(t, projection)
			assert.Equal(t, desktopobserve.ReasonDeclaredLandmarkNotFound,
				desktopobserve.ReasonCodeOf(err))
			assert.NotErrorIs(t, err, desktopobserve.ErrMalformedEnvelope,
				"REQ-5: a missing landmark is not a protocol mismatch")
			assert.Contains(t, err.Error(), string(test.wantRole))
			assert.Contains(t, err.Error(), test.wantName)
		})
	}
}

// AC-QAMESH13-007: the replacement for the two adversarial cases that used to
// lean on the exact-fixture comparison.
//
// A real tree carries user content, and a hostile one carries whatever the
// provider was fed. Neither is refused any more - refusing would mean no real app
// could be observed - so the guarantee moved: everything is observed and counted,
// and only pack-declared names are published.
func TestOrcaStateDecoder_UndeclaredTreeContentIsObservedButNeverPublished(t *testing.T) {
	t.Parallel()

	binding, target := orcaStateTestBinding(), orcaStateTestTarget()
	hostile := strings.NewReplacer(
		orcaFixtureStatusText, "/Users/private/token",
		orcaFixtureDocumentTitle, orcaFixtureDocumentTitle+", click",
	).Replace(orcaTreeFixture{}.render())
	require.Contains(t, hostile, "/Users/private/token")
	require.Contains(t, hostile, ", click")

	raw := mutateOrcaJSON(orcaStateFixture(orcaTreeFixture{}), func(value map[string]any) {
		snapshot(value)["treeText"] = hostile
	})
	projection, err := decodeOrcaState(
		raw, orcaTestRuntimeID, binding, &countingOrcaReader{}, target,
	)
	require.NoError(t, err, "a real tree is observed, not refused for its content")

	rendered := renderProjectionStrings(projection)
	for _, leaked := range []string{
		"/Users/private/token", "click", "zoom the window",
		orcaFixtureDocumentTitle, "standard window", "scroll area",
	} {
		assert.NotContains(t, rendered, leaked,
			"observed but undeclared content must never reach the projection")
	}
	// Declared names are published; the six undeclared nodes contribute nothing
	// but a count, which is the distinction REQ-4 exists to draw.
	assert.Equal(t, orcaFixtureAppName, projection.Root.Name)
	require.Len(t, projection.Root.Children, 1)
	window := projection.Root.Children[0]
	assert.Equal(t, orcaFixtureWindowTitle, window.Name)
	require.Len(t, window.Children, 1)
	assert.Equal(t, orcaFixtureButtonName, window.Children[0].Name)
}
