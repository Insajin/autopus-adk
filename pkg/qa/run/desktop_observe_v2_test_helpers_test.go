package run

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type desktopV2TestNode struct {
	AdvertisedActions []string              `json:"advertised_actions"`
	Children          []desktopV2TestNode   `json:"children"`
	Frame             *desktopobserve.Frame `json:"frame,omitempty"`
	Name              string                `json:"name"`
	NodeRef           string                `json:"node_ref"`
	Occurrence        uint64                `json:"occurrence"`
	ParentNodeRef     *string               `json:"parent_node_ref"`
	Role              string                `json:"role"`
	SemanticState     map[string]any        `json:"semantic_state"`
}

type desktopV2TestProjection struct {
	SchemaVersion string              `json:"schema_version"`
	ProviderRef   string              `json:"provider_ref"`
	AppRef        string              `json:"app_ref"`
	WindowRef     string              `json:"window_ref"`
	StateRef      string              `json:"state_ref"`
	Digest        string              `json:"digest"`
	Nodes         []desktopV2TestNode `json:"nodes"`
}

func desktopV2PublicRequest(
	t *testing.T,
	operation desktopobserve.Operation,
) ([]byte, desktopV2Binding) {
	t.Helper()
	scope := desktopobserve.ReceiptScope{
		Kind: desktopobserve.ScopeProvider, PublicRef: "autopus-desktop-local",
	}
	if operation == desktopobserve.OperationListWindows {
		scope = desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeApplication, PublicRef: "autopus-desktop"}
	} else if operation == desktopobserve.OperationGetState {
		scope = desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeWindow, PublicRef: "main-window"}
	}
	request := desktopobserve.Request{
		ProtocolVersion: 1, RequestID: "desktop-observe-golden", Operation: operation, Scope: scope,
	}
	raw, err := json.Marshal(request)
	require.NoError(t, err)
	raw = append(raw, '\n')
	_, binding, err := encodeDesktopV2Request(raw)
	require.NoError(t, err)
	return raw, binding
}

func desktopV2SuccessResult(t *testing.T, binding desktopV2Binding, payload any) []byte {
	t.Helper()
	redaction := "not_required"
	if binding.publicRequest.Operation == desktopobserve.OperationGetState {
		redaction = "applied"
	}
	receipt := desktopV2Receipt{
		SchemaVersion:     desktopobserve.RuntimeReceiptSchemaVersion,
		Provider:          desktopV2Provider{Name: "rust-go", Version: "0.0.1", ProtocolVersion: 2},
		Scope:             binding.privateRequest.Scope,
		CapabilitySummary: desktopV2TestCapabilities(),
		Redaction:         desktopV2Status{Status: redaction}, Quarantine: desktopV2Status{Status: "empty"},
	}
	result := struct {
		ProtocolVersion int              `json:"protocol_version"`
		RequestID       string           `json:"request_id"`
		Status          string           `json:"status"`
		RuntimeReceipt  desktopV2Receipt `json:"runtime_receipt"`
		Payload         any              `json:"payload"`
	}{2, binding.privateRequest.RequestID, desktopV2StatusOK, receipt, payload}
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	return append(raw, '\n')
}

func desktopV2FailureResult(
	t *testing.T,
	binding desktopV2Binding,
	reason desktopobserve.ReasonCode,
) []byte {
	t.Helper()
	reasonValue := string(reason)
	nextStep := desktopV2NextSteps[reason]
	receipt := desktopV2Receipt{
		SchemaVersion: desktopobserve.RuntimeReceiptSchemaVersion,
		Provider:      desktopV2Provider{Name: "rust-go", Version: "0.0.1", ProtocolVersion: 2},
		Scope:         binding.privateRequest.Scope, CapabilitySummary: desktopV2TestCapabilities(),
		ReasonCode: &reasonValue, NextStep: &nextStep,
		Redaction: desktopV2Status{Status: "not_required"}, Quarantine: desktopV2Status{Status: "empty"},
	}
	result := struct {
		ProtocolVersion int              `json:"protocol_version"`
		RequestID       string           `json:"request_id"`
		Status          string           `json:"status"`
		RuntimeReceipt  desktopV2Receipt `json:"runtime_receipt"`
	}{2, binding.privateRequest.RequestID, desktopV2StatusError, receipt}
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	return append(raw, '\n')
}

func desktopV2TestCapabilities() []desktopV2Capability {
	result := make([]desktopV2Capability, 0, len(desktopV2Operations))
	for _, operation := range desktopV2Operations {
		result = append(result, desktopV2Capability{Name: operation, Status: "supported"})
	}
	return result
}

func desktopV2ProjectionResult(t *testing.T, root desktopV2TestNode) json.RawMessage {
	t.Helper()
	wire := desktopV2TestProjection{
		SchemaVersion: desktopV2ProjectionSchema, ProviderRef: "provider_rust_go",
		AppRef: "autopus-desktop", WindowRef: "main-window",
		StateRef: "state_v2_" + repeatDesktopV2Zero(),
		Digest:   repeatDesktopV2Zero(), Nodes: []desktopV2TestNode{root},
	}
	raw, err := json.Marshal(wire)
	require.NoError(t, err)
	projection, err := decodeDesktopV2Projection(raw)
	require.NoError(t, err)
	canonical, err := desktopV2CanonicalBytes(projection)
	require.NoError(t, err)
	digest := sha256.Sum256(canonical)
	wire.Digest = hex.EncodeToString(digest[:])
	raw, err = json.Marshal(wire)
	require.NoError(t, err)
	return raw
}

func repeatDesktopV2Zero() string {
	return "0000000000000000000000000000000000000000000000000000000000000000"
}

func desktopV2RecursiveRoot(expanded bool) desktopV2TestNode {
	applicationRef := "application_fixture_00"
	windowRef := "window_fixture_00"
	groupRef := "group_fixture_00"
	disclosureOne := desktopV2TestNode{
		AdvertisedActions: []string{"AXPress"}, Children: []desktopV2TestNode{}, Name: "Disclosure",
		NodeRef: "disclosure_fixture_00", ParentNodeRef: &groupRef, Role: "AXButton",
		SemanticState: map[string]any{"enabled": true, "expanded": expanded},
	}
	disclosureTwo := disclosureOne
	disclosureTwo.NodeRef = "disclosure_fixture_01"
	disclosureTwo.Occurrence = 1
	group := desktopV2TestNode{
		AdvertisedActions: []string{}, Children: []desktopV2TestNode{disclosureOne, disclosureTwo},
		Name: "Disclosure", NodeRef: groupRef, ParentNodeRef: &windowRef, Role: "AXGroup",
		SemanticState: map[string]any{"selected": false},
	}
	status := desktopV2TestNode{
		AdvertisedActions: []string{}, Children: []desktopV2TestNode{}, Name: "Status",
		NodeRef: "status_fixture_00", ParentNodeRef: &windowRef, Role: "AXStaticText",
		SemanticState: map[string]any{"visible": true},
	}
	window := desktopV2TestNode{
		AdvertisedActions: []string{}, Children: []desktopV2TestNode{group, status}, Name: "Autopus",
		NodeRef: windowRef, ParentNodeRef: &applicationRef, Role: "AXWindow",
		SemanticState: map[string]any{"focused": true},
	}
	return desktopV2TestNode{
		AdvertisedActions: []string{}, Children: []desktopV2TestNode{window}, Name: "Autopus",
		NodeRef: applicationRef, ParentNodeRef: nil, Role: "AXApplication",
		SemanticState: map[string]any{"enabled": true},
	}
}
