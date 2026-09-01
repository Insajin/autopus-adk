package run

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

// SPEC-QAMESH-013: the receipt scopes come from the pack's refs.
//
// This was the fourth and last pin layer. Every scope the runner published was
// built from the literals "autopus-desktop" and "main-window", so a pack naming
// its own refs produced a receipt describing one window and a projection
// describing another, and publication refused it as a scope binding mismatch -
// after the pack had validated, the provider had run, and the tree had parsed.
func TestDesktopRunner_PublishesPackDeclaredScopes(t *testing.T) {
	t.Parallel()

	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	projection := runnerSemanticFixture("provider-local", "state-thirdparty")
	projection.AppRef = "thirdparty-app"
	projection.WindowRef = "primary-window"
	local.projection = &projection
	local.apps = []desktopobserve.AppSummary{{AppRef: "thirdparty-app"}}
	local.windows = []desktopobserve.WindowSummary{{WindowRef: "primary-window"}}

	request := desktopRunRequest(desktopobserve.RuntimeProviderLocal)
	request.AppRef = "thirdparty-app"
	request.WindowRef = "primary-window"

	outcome, err := newDesktopObservationRunner(
		local, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
	).Run(context.Background(), request)
	require.NoError(t, err)
	require.Nil(t, outcome.ReasonCode, "a third-party pack must reach a verdict, not a scope refusal")
	assert.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
	assert.Equal(t, desktopobserve.ReceiptScope{
		Kind: desktopobserve.ScopeWindow, PublicRef: "primary-window",
	}, outcome.RuntimeReceipt.Scope)

	// The published evidence has to survive the same decode-and-validate that
	// refused the live run, which is where the hardcoded "main-window" scope was
	// compared against the projection's real window ref.
	evidence := desktopObservationEvidence(outcome)
	require.NotNil(t, evidence.SemanticProjection)
	assert.Equal(t, "primary-window", evidence.SemanticProjection.WindowRef)
	body, err := json.Marshal(evidence)
	require.NoError(t, err)
	decoded, err := desktopobserve.DecodeObservationEvidence(body)
	require.NoError(t, err, "this is the scope binding mismatch the live run hit")
	require.NotNil(t, decoded.SemanticProjection)
	assert.Equal(t, "thirdparty-app", decoded.SemanticProjection.AppRef)
	assert.Equal(t, "primary-window", decoded.SemanticProjection.WindowRef)
}

// The app ref the projection reports is bound against the request's, at the layer
// that knows both. This replaces the "projection app" case retired from
// desktopobserve's protocol binding table, where the comparison was against a
// compiled-in ref rather than the pack's.
func TestDesktopRunner_ProjectionRefDriftFailsClosed(t *testing.T) {
	t.Parallel()

	for name, drift := range map[string]func(*desktopobserve.SemanticProjection){
		"app ref drift":      func(p *desktopobserve.SemanticProjection) { p.AppRef = "other-app" },
		"window ref drift":   func(p *desktopobserve.SemanticProjection) { p.WindowRef = "other-window" },
		"provider ref drift": func(p *desktopobserve.SemanticProjection) { p.ProviderRef = "provider-orca" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
			projection := runnerSemanticFixture("provider-local", "state-drift")
			drift(&projection)
			local.projection = &projection

			outcome, err := newDesktopObservationRunner(
				local, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
			).Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
			require.NoError(t, err)
			assert.NotEqual(t, desktopobserve.VerdictPassed, outcome.Verdict)
			assert.Nil(t, outcome.SemanticProjection)
		})
	}
}

// An unset ref refuses instead of matching. Every scope is built from these two
// refs now, and two empty refs compare equal - so a scope built from nothing
// would accept anything, which is the pin behaviour under a new name.
func TestDesktopRunner_UnsetRefsRefuseBeforeAddressingAProvider(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*DesktopObservationRunRequest){
		"no app ref":                  func(request *DesktopObservationRunRequest) { request.AppRef = "" },
		"no window ref":               func(request *DesktopObservationRunRequest) { request.WindowRef = "" },
		"neither ref":                 func(request *DesktopObservationRunRequest) { request.AppRef, request.WindowRef = "", "" },
		"app ref is not alias-shaped": func(request *DesktopObservationRunRequest) { request.AppRef = "Third Party.App" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
			request := desktopRunRequest(desktopobserve.RuntimeProviderLocal)
			mutate(&request)

			outcome, err := newDesktopObservationRunner(
				local, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
			).Run(context.Background(), request)
			require.NoError(t, err)
			assert.NotEqual(t, desktopobserve.VerdictPassed, outcome.Verdict)
			assert.Nil(t, outcome.SemanticProjection)
			assert.Empty(t, local.calls, "no provider is addressed under an alias nobody declared")
		})
	}
}
