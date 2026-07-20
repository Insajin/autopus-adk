package desktopobserve

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeExchange_GetStateBindsRequestProjectionReceiptAndProvider(t *testing.T) {
	t.Parallel()

	request := []byte(`{"protocol_version":1,"request_id":"req-bind","operation":"get_state","scope":{"kind":"window","public_ref":"main-window"}}`)
	tests := []struct {
		name   string
		mutate func(*SemanticProjection, *RuntimeReceipt)
	}{
		{name: "projection window", mutate: func(projection *SemanticProjection, _ *RuntimeReceipt) { projection.WindowRef = "other-window" }},
		{name: "projection app", mutate: func(projection *SemanticProjection, _ *RuntimeReceipt) { projection.AppRef = "other-app" }},
		{name: "projection provider", mutate: func(projection *SemanticProjection, _ *RuntimeReceipt) { projection.ProviderRef = "provider-orca" }},
		{name: "receipt state", mutate: func(_ *SemanticProjection, receipt *RuntimeReceipt) {
			receipt.Scope = ReceiptScope{Kind: ScopeState, PublicRef: "state-other"}
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			projection := semanticFixture()
			receipt := successfulStateReceipt(RuntimeProviderLocal)
			test.mutate(&projection, &receipt)
			normalized, err := NormalizeProjection(projection, identityRedactor)
			require.NoError(t, err)
			result := passedResultEnvelope(t, "req-bind", map[string]any{"semantic_projection": normalized}, receipt)

			_, err = DecodeExchange(request, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrScopeMismatch)
		})
	}
}

func TestDecodeExchange_GetStateRequiresAppliedRedactionProof(t *testing.T) {
	t.Parallel()

	projection, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	receipt := successfulReceipt(RuntimeProviderLocal)
	result := passedResultEnvelope(t, "req-redaction", map[string]any{"semantic_projection": projection}, receipt)
	request := []byte(`{"protocol_version":1,"request_id":"req-redaction","operation":"get_state","scope":{"kind":"window","public_ref":"main-window"}}`)

	_, err = DecodeExchange(request, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRedactionFailed)
}

func TestDecodeExchange_GetStateAcceptsBoundLocalAndOrcaProviders(t *testing.T) {
	t.Parallel()

	for _, provider := range []RuntimeProvider{RuntimeProviderLocal, RuntimeProviderOrca} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()
			projection := semanticFixture()
			projection.ProviderRef = providerRef(providerIdentity(provider).Name)
			projection.StateRef = "state-bound-" + string(provider)
			normalized, err := NormalizeProjection(projection, identityRedactor)
			require.NoError(t, err)
			receipt := successfulStateReceipt(provider)
			receipt.Scope = ReceiptScope{Kind: ScopeState, PublicRef: normalized.StateRef}
			result := passedResultEnvelope(
				t, "req-provider", map[string]any{"semantic_projection": normalized}, receipt,
			)
			request := []byte(`{"protocol_version":1,"request_id":"req-provider","operation":"get_state","scope":{"kind":"window","public_ref":"main-window"}}`)

			exchange, err := DecodeExchange(request, result)
			require.NoError(t, err)
			assert.Equal(t, providerIdentity(provider), exchange.Result.RuntimeReceipt.Provider)
		})
	}
}

func TestDecodeExchange_PassedPayloadCannotRepresentFailureState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation Operation
		scope     ReceiptScope
		payload   any
	}{
		{name: "permission false", operation: OperationPermissions, scope: ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"}, payload: map[string]any{"accessibility_granted": false}},
		{name: "apps empty", operation: OperationListApps, scope: ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"}, payload: map[string]any{"apps": []AppSummary{}}},
		{name: "apps duplicate", operation: OperationListApps, scope: ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"}, payload: map[string]any{"apps": []AppSummary{{AppRef: "autopus-desktop"}, {AppRef: "autopus-desktop"}}}},
		{name: "windows empty", operation: OperationListWindows, scope: ReceiptScope{Kind: ScopeApplication, PublicRef: "autopus-desktop"}, payload: map[string]any{"windows": []WindowSummary{}}},
		{name: "windows duplicate", operation: OperationListWindows, scope: ReceiptScope{Kind: ScopeApplication, PublicRef: "autopus-desktop"}, payload: map[string]any{"windows": []WindowSummary{{WindowRef: "main-window"}, {WindowRef: "main-window"}}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := []byte(fmt.Sprintf(
				`{"protocol_version":1,"request_id":"req-failure","operation":%q,"scope":{"kind":%q,"public_ref":%q}}`,
				test.operation, test.scope.Kind, test.scope.PublicRef,
			))
			receipt := successfulReceiptForScope(RuntimeProviderLocal, test.scope)
			result := passedResultEnvelope(t, "req-failure", test.payload, receipt)

			_, err := DecodeExchange(request, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrMalformedEnvelope)
		})
	}
}

func passedResultEnvelope(t *testing.T, requestID string, payload any, receipt RuntimeReceipt) []byte {
	t.Helper()
	payloadBody, err := json.Marshal(payload)
	require.NoError(t, err)
	receiptBody, err := json.Marshal(receipt)
	require.NoError(t, err)
	return []byte(fmt.Sprintf(
		`{"protocol_version":1,"request_id":%q,"status":"passed","payload":%s,"runtime_receipt":%s}`,
		requestID, payloadBody, receiptBody,
	))
}
