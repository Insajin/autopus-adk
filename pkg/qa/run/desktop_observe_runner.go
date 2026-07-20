package run

import (
	"context"
	"errors"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func (runner *desktopObservationRunner) Run(
	ctx context.Context,
	request DesktopObservationRunRequest,
) (desktopobserve.OracleOutcome, error) {
	if err := validDesktopProvider(request.RuntimeProvider); err != nil {
		return desktopobserve.OracleOutcome{}, err
	}
	expectedIdentity := expectedDesktopIdentity(request.RuntimeProvider)
	if !validDesktopOperations(request.Operations) {
		return desktopFailureOutcome(
			desktopobserve.FailureOperationMissing,
			expectedIdentity,
			desktopProviderScope(expectedIdentity),
			desktopUnsupportedCapabilities(),
			desktopobserve.OperationCapabilities,
		)
	}
	ctx, cancel := runner.operationContext(ctx)
	defer cancel()

	client, err := runner.resolveClient(ctx, request.RuntimeProvider)
	err = desktopBoundedError(ctx, err)
	if err != nil || client == nil {
		return desktopFailureOutcome(
			desktopobserve.FailureProviderStart,
			expectedIdentity,
			desktopProviderScope(expectedIdentity),
			desktopUnsupportedCapabilities(),
			"",
		)
	}
	identity, err := client.Handshake(ctx)
	err = desktopBoundedError(ctx, err)
	if err != nil {
		return desktopFailureFromError(
			err, expectedIdentity, desktopProviderScope(expectedIdentity), desktopUnsupportedCapabilities(),
		)
	}
	if !validDesktopIdentity(identity, request.RuntimeProvider) {
		return desktopFailureOutcome(
			desktopobserve.FailureProtocolVersion,
			expectedIdentity,
			desktopProviderScope(expectedIdentity),
			desktopUnsupportedCapabilities(),
			"",
		)
	}

	operations, err := client.Capabilities(ctx)
	err = desktopBoundedError(ctx, err)
	if err != nil {
		return desktopFailureFromError(
			err, identity, desktopProviderScope(identity), desktopUnsupportedCapabilities(),
		)
	}
	capabilities := desktopCapabilitySummary(operations)
	if missing := desktopFirstMissingCapability(capabilities); missing != "" {
		return desktopFailureOutcome(
			desktopobserve.FailureOperationMissing,
			identity,
			desktopProviderScope(identity),
			capabilities,
			missing,
		)
	}

	permission, err := client.Permissions(ctx)
	err = desktopBoundedError(ctx, err)
	if err != nil {
		return desktopFailureFromError(err, identity, desktopProviderScope(identity), capabilities)
	}
	if !permission.AccessibilityGranted {
		return desktopFailureOutcome(
			desktopobserve.FailureAccessibilityDenied,
			identity,
			desktopProviderScope(identity),
			capabilities,
			"",
		)
	}

	apps, err := client.ListApps(ctx)
	err = desktopBoundedError(ctx, err)
	if err != nil {
		return desktopFailureFromError(err, identity, desktopProviderScope(identity), capabilities)
	}
	appMatched, multipleApps := desktopSingleAppMatches(apps, request.AppRef)
	if multipleApps {
		return desktopFailureOutcome(
			desktopobserve.FailureProtocolVersion,
			identity,
			desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeApplication, PublicRef: "autopus-desktop"},
			capabilities,
			"",
		)
	}
	if !appMatched {
		return desktopFailureOutcome(
			desktopobserve.FailureAppAliasUnmatched,
			identity,
			desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeApplication, PublicRef: "autopus-desktop"},
			capabilities,
			"",
		)
	}

	windows, err := client.ListWindows(ctx, request.AppRef)
	err = desktopBoundedError(ctx, err)
	if err != nil {
		return desktopFailureFromError(
			err,
			identity,
			desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeApplication, PublicRef: "autopus-desktop"},
			capabilities,
		)
	}
	windowMatched, multipleWindows := desktopSingleWindowMatches(windows, request.WindowRef)
	if multipleWindows {
		return desktopFailureOutcome(
			desktopobserve.FailureProtocolVersion,
			identity,
			desktopWindowScope("main-window"),
			capabilities,
			"",
		)
	}
	if !windowMatched {
		return desktopFailureOutcome(
			desktopobserve.FailureWindowAliasUnmatched,
			identity,
			desktopWindowScope("main-window"),
			capabilities,
			"",
		)
	}

	projection, err := client.GetState(ctx, request.AppRef, request.WindowRef)
	err = desktopBoundedError(ctx, err)
	if err != nil {
		return desktopFailureFromError(err, identity, desktopWindowScope("main-window"), capabilities)
	}
	if !desktopProjectionMatchesRequest(projection, request) {
		return desktopFailureOutcome(
			desktopobserve.FailureProtocolVersion,
			identity,
			desktopWindowScope("main-window"),
			capabilities,
			"",
		)
	}
	projection, err = desktopobserve.NormalizeProjection(projection, request.Redactor)
	if err != nil {
		return desktopFailureFromError(err, identity, desktopWindowScope("main-window"), capabilities)
	}
	return runner.evaluateProjection(request, identity, capabilities, projection)
}

func (runner *desktopObservationRunner) evaluateProjection(
	request DesktopObservationRunRequest,
	identity desktopobserve.ProviderIdentity,
	capabilities []desktopobserve.CapabilityStatus,
	projection desktopobserve.SemanticProjection,
) (desktopobserve.OracleOutcome, error) {
	binding := desktopobserve.StateBinding{
		StateRef: projection.StateRef, ProviderRef: projection.ProviderRef,
		AppRef: projection.AppRef, WindowRef: projection.WindowRef, Digest: projection.Digest,
	}
	ledger := runner.ledgers[request.RuntimeProvider]
	if ledger == nil {
		return desktopobserve.OracleOutcome{}, errors.New("desktop observation ledger is unavailable")
	}
	if err := ledger.Register(binding); err != nil {
		return desktopFailureOutcome(
			desktopobserve.FailureStateRefRejected,
			identity,
			desktopWindowScope("main-window"),
			capabilities,
			"",
		)
	}
	if !desktopHasStructuralLandmarks(projection.Root, request.Policy.MinimumLandmarks) {
		_ = ledger.Consume(binding)
		return desktopFailureOutcome(
			desktopobserve.FailureLandmarksInsufficient,
			identity,
			desktopWindowScope("main-window"),
			capabilities,
			"",
		)
	}
	receipt := desktopSuccessReceipt(identity, desktopWindowScope("main-window"), capabilities)
	return desktopobserve.EvaluateOracle(desktopobserve.OracleInput{
		Projection: projection,
		Ledger:     ledger,
		Policy:     request.Policy,
		Receipt:    receipt,
	})
}

func desktopHasStructuralLandmarks(
	root desktopobserve.SemanticNode,
	requirements []desktopobserve.LandmarkRequirement,
) bool {
	if len(requirements) == 0 {
		return true
	}
	for _, requirement := range requirements {
		if !desktopContainsStructuralLandmark(root, requirement) {
			return false
		}
	}
	return true
}

func desktopContainsStructuralLandmark(
	node desktopobserve.SemanticNode,
	requirement desktopobserve.LandmarkRequirement,
) bool {
	if node.Role == requirement.Role && desktopStateIsTrue(node.SemanticState, requirement.RequiredState) {
		return true
	}
	for _, child := range node.Children {
		if desktopContainsStructuralLandmark(child, requirement) {
			return true
		}
	}
	return false
}

func desktopStateIsTrue(state desktopobserve.SemanticState, key desktopobserve.SemanticStateKey) bool {
	var value *bool
	switch key {
	case desktopobserve.StateEnabled:
		value = state.Enabled
	case desktopobserve.StateFocused:
		value = state.Focused
	case desktopobserve.StateSelected:
		value = state.Selected
	case desktopobserve.StateExpanded:
		value = state.Expanded
	default:
		return false
	}
	return value != nil && *value
}

func desktopProjectionMatchesRequest(
	projection desktopobserve.SemanticProjection,
	request DesktopObservationRunRequest,
) bool {
	return projection.ProviderRef == desktopProviderPublicRef(request.RuntimeProvider) &&
		projection.AppRef == request.AppRef && projection.WindowRef == request.WindowRef &&
		strings.HasPrefix(projection.StateRef, "state-")
}

func desktopSingleAppMatches(apps []desktopobserve.AppSummary, appRef string) (bool, bool) {
	if len(apps) > 1 {
		return false, true
	}
	return len(apps) == 1 && apps[0].AppRef == appRef, false
}

func desktopSingleWindowMatches(windows []desktopobserve.WindowSummary, windowRef string) (bool, bool) {
	if len(windows) > 1 {
		return false, true
	}
	return len(windows) == 1 && windows[0].WindowRef == windowRef, false
}
