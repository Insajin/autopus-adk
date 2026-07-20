package run

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

type fakeDesktopClient struct {
	provider             desktopobserve.RuntimeProvider
	protocolVersion      int
	handshakeErr         error
	accessibilityGranted bool
	capabilities         []desktopobserve.Operation
	apps                 []desktopobserve.AppSummary
	windows              []desktopobserve.WindowSummary
	projection           *desktopobserve.SemanticProjection
	getStateErr          error
	calls                []string
	rawIndexCalls        int
	actionCalls          int
	screenshotCalls      int
}

func newFakeDesktopClient(provider desktopobserve.RuntimeProvider) *fakeDesktopClient {
	return &fakeDesktopClient{
		provider: provider, protocolVersion: desktopobserve.ProtocolVersion, accessibilityGranted: true,
	}
}

func (client *fakeDesktopClient) Handshake(context.Context) (desktopobserve.ProviderIdentity, error) {
	client.calls = append(client.calls, "handshake")
	name := "autopus-desktop-local"
	if client.provider == desktopobserve.RuntimeProviderOrca {
		name = "orca-computer-use-macos"
	}
	return desktopobserve.ProviderIdentity{
		Name: name, Version: "1.0.0", ProtocolVersion: client.protocolVersion,
	}, client.handshakeErr
}

func (client *fakeDesktopClient) Capabilities(context.Context) ([]desktopobserve.Operation, error) {
	client.calls = append(client.calls, "capabilities")
	if client.capabilities != nil {
		return append([]desktopobserve.Operation{}, client.capabilities...), nil
	}
	return append(desktopobserve.ReadOnlyOperations(), desktopobserve.Operation("screenshot")), nil
}

func (client *fakeDesktopClient) Permissions(context.Context) (desktopobserve.PermissionResult, error) {
	client.calls = append(client.calls, "permissions")
	return desktopobserve.PermissionResult{AccessibilityGranted: client.accessibilityGranted}, nil
}

func (client *fakeDesktopClient) ListApps(context.Context) ([]desktopobserve.AppSummary, error) {
	client.calls = append(client.calls, "list_apps")
	if client.apps != nil {
		return append([]desktopobserve.AppSummary{}, client.apps...), nil
	}
	return []desktopobserve.AppSummary{{AppRef: "autopus-desktop"}}, nil
}

func (client *fakeDesktopClient) ListWindows(context.Context, string) ([]desktopobserve.WindowSummary, error) {
	client.calls = append(client.calls, "list_windows")
	if client.windows != nil {
		return append([]desktopobserve.WindowSummary{}, client.windows...), nil
	}
	return []desktopobserve.WindowSummary{{WindowRef: "main-window"}}, nil
}

func (client *fakeDesktopClient) GetState(context.Context, string, string) (desktopobserve.SemanticProjection, error) {
	client.calls = append(client.calls, "get_state")
	if client.getStateErr != nil {
		return desktopobserve.SemanticProjection{}, client.getStateErr
	}
	if client.projection != nil {
		return *client.projection, nil
	}
	providerRef := "provider-local"
	if client.provider == desktopobserve.RuntimeProviderOrca {
		providerRef = "provider-orca"
	}
	return runnerSemanticFixture(providerRef, "state-shared"), nil
}

func (client *fakeDesktopClient) ResolveRawIndex(context.Context, int) error {
	client.rawIndexCalls++
	return nil
}

func (client *fakeDesktopClient) InvokeAction(context.Context, string) error {
	client.actionCalls++
	return nil
}

func (client *fakeDesktopClient) Screenshot(context.Context) error {
	client.screenshotCalls++
	return nil
}

func desktopRunRequest(provider desktopobserve.RuntimeProvider) DesktopObservationRunRequest {
	return DesktopObservationRunRequest{
		RuntimeProvider: provider,
		Operations: []desktopobserve.Operation{
			desktopobserve.OperationCapabilities,
			desktopobserve.OperationPermissions,
			desktopobserve.OperationListApps,
			desktopobserve.OperationListWindows,
			desktopobserve.OperationGetState,
		},
		AppRef:    "autopus-desktop",
		WindowRef: "main-window",
		Policy: desktopobserve.OraclePolicy{
			AllowedNames: []string{"Autopus"},
			MinimumLandmarks: []desktopobserve.LandmarkRequirement{
				{Role: desktopobserve.RoleApplication, Name: "Autopus", RequiredState: desktopobserve.StateEnabled},
				{Role: desktopobserve.RoleWindow, Name: "Autopus", RequiredState: desktopobserve.StateFocused},
			},
		},
		Redactor: func(value string) (string, error) { return value, nil },
	}
}

func runnerSemanticFixture(providerRef, stateRef string) desktopobserve.SemanticProjection {
	enabled, focused := true, true
	return desktopobserve.SemanticProjection{
		SchemaVersion: desktopobserve.SemanticProjectionSchemaVersion,
		ProviderRef:   providerRef,
		AppRef:        "autopus-desktop",
		WindowRef:     "main-window",
		StateRef:      stateRef,
		Root: desktopobserve.SemanticNode{
			Role:              desktopobserve.RoleApplication,
			Name:              "Autopus",
			SemanticState:     desktopobserve.SemanticState{Enabled: &enabled},
			AdvertisedActions: []desktopobserve.Action{desktopobserve.ActionShowMenu, desktopobserve.ActionPress},
			Children: []desktopobserve.SemanticNode{{
				Role:          desktopobserve.RoleWindow,
				Name:          "Autopus",
				SemanticState: desktopobserve.SemanticState{Focused: &focused},
			}},
		},
	}
}

func desktopObservationPack() journey.Pack {
	return journey.Pack{
		ID: "desktop-accessibility-observe", Surface: "desktop", Lanes: []string{"desktop-native"},
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
			SourceSpec: "SPEC-QAMESH-012", AcceptanceRefs: []string{"AC-QAMESH12-011"},
		},
		PassFailAuthority: "deterministic",
	}
}

func capabilityNames(receipt desktopobserve.RuntimeReceipt) []desktopobserve.Operation {
	out := make([]desktopobserve.Operation, 0, len(receipt.CapabilitySummary))
	for _, capability := range receipt.CapabilitySummary {
		out = append(out, capability.Name)
	}
	return out
}
