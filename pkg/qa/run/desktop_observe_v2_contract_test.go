package run

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopV2RequestAdapter_MapsExactIDsScopesAndOperations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		operation desktopobserve.Operation
		scope     desktopV2Scope
	}{
		{desktopobserve.OperationCapabilities, desktopV2Scope{"provider", "provider_selected"}},
		{desktopobserve.OperationPermissions, desktopV2Scope{"provider", "provider_selected"}},
		{desktopobserve.OperationListApps, desktopV2Scope{"provider", "provider_selected"}},
		{desktopobserve.OperationListWindows, desktopV2Scope{"application", "autopus-desktop"}},
		{desktopobserve.OperationGetState, desktopV2Scope{"window", "main-window"}},
	}
	for _, test := range tests {
		t.Run(string(test.operation), func(t *testing.T) {
			publicRaw, _ := desktopV2PublicRequest(t, test.operation)
			privateRaw, binding, err := encodeDesktopV2Request(publicRaw)
			require.NoError(t, err)
			assert.Equal(t, 2, binding.privateRequest.ProtocolVersion)
			assert.Equal(t, string(test.operation), binding.privateRequest.Operation)
			assert.Equal(t, test.scope, binding.privateRequest.Scope)
			assert.Regexp(t, `^req_adk_[0-9a-f]{64}$`, binding.privateRequest.RequestID)
			assert.LessOrEqual(t, len(privateRaw), desktopobserve.MaxEnvelopeBytes)
			assert.Equal(t, byte('\n'), privateRaw[len(privateRaw)-1])
		})
	}
}

func TestDesktopV2RequestAdapter_MapsOnlyExactPrivateStateTokens(t *testing.T) {
	t.Parallel()
	request := desktopobserve.Request{
		ProtocolVersion: 1, RequestID: "desktop-observe-state", Operation: desktopobserve.OperationGetState,
		Scope:            desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeWindow, PublicRef: "main-window"},
		ExpectedStateRef: "state-" + repeatDesktopV2Zero(),
	}
	raw, err := json.Marshal(request)
	require.NoError(t, err)
	privateRaw, binding, err := encodeDesktopV2Request(append(raw, '\n'))
	require.NoError(t, err)
	assert.Equal(t, "state_v2_"+repeatDesktopV2Zero(), binding.privateRequest.ExpectedStateRef)
	assert.Contains(t, string(privateRaw), `"expected_state_ref":"state_v2_`)

	request.ExpectedStateRef = "state-" + strings.Repeat("z", 64)
	raw, err = json.Marshal(request)
	require.NoError(t, err)
	privateRaw, _, err = encodeDesktopV2Request(append(raw, '\n'))
	assert.Error(t, err)
	assert.Nil(t, privateRaw)
}

func TestDesktopV2ResultAdapter_GoldenCapabilitiesAndGetState(t *testing.T) {
	t.Parallel()
	t.Run("capabilities", func(t *testing.T) {
		publicRaw, binding := desktopV2PublicRequest(t, desktopobserve.OperationCapabilities)
		privateRaw := desktopV2SuccessResult(t, binding, map[string]any{
			"capabilities": desktopV2Operations,
		})
		resultRaw, status, err := translateDesktopV2Result(binding, privateRaw)
		require.NoError(t, err)
		assert.Equal(t, desktopV2StatusOK, status)
		exchange, err := desktopobserve.DecodeExchange(publicRaw, resultRaw)
		require.NoError(t, err)
		assert.Equal(t, "autopus-desktop-local", exchange.Result.RuntimeReceipt.Provider.Name)
		assert.Equal(t, "0.0.1", exchange.Result.RuntimeReceipt.Provider.Version)
		assert.NotContains(t, string(resultRaw), "rust-go")
	})

	t.Run("recursive get state", func(t *testing.T) {
		publicRaw, binding := desktopV2PublicRequest(t, desktopobserve.OperationGetState)
		privateProjection := desktopV2ProjectionResult(t, desktopV2RecursiveRoot(false))
		privateRaw := desktopV2SuccessResult(t, binding, map[string]any{
			"semantic_projection": privateProjection,
		})
		resultRaw, _, err := translateDesktopV2Result(binding, privateRaw)
		require.NoError(t, err)
		exchange, err := desktopobserve.DecodeExchange(publicRaw, resultRaw)
		require.NoError(t, err)
		var payload struct {
			Projection desktopobserve.SemanticProjection `json:"semantic_projection"`
		}
		require.NoError(t, json.Unmarshal(exchange.Result.Payload, &payload))
		assert.Equal(t, "provider-local", payload.Projection.ProviderRef)
		assert.Equal(t, "state-"+repeatDesktopV2Zero(), payload.Projection.StateRef)
		assert.Regexp(t, `^[0-9a-f]{64}$`, payload.Projection.Digest)
		assert.Regexp(t, `^n_[0-9a-f]{64}$`, payload.Projection.Root.NodeRef)
		assert.Equal(t, 3, countPublicDesktopNodesByName(payload.Projection.Root, "Disclosure"))
		assert.False(t, strings.Contains(string(resultRaw), "fixture_00"))
	})
}

func TestDesktopV2ResultAdapter_RejectsMalformedBoundaries(t *testing.T) {
	t.Parallel()
	_, binding := desktopV2PublicRequest(t, desktopobserve.OperationCapabilities)
	valid := desktopV2SuccessResult(t, binding, map[string]any{"capabilities": desktopV2Operations})
	tests := []struct {
		name string
		raw  []byte
	}{
		{"duplicate top key", bytes.Replace(valid, []byte(`"status":"ok"`), []byte(`"status":"ok","status":"ok"`), 1)},
		{"duplicate nested key", bytes.Replace(valid, []byte(`"name":"rust-go"`), []byte(`"name":"rust-go","name":"rust-go"`), 1)},
		{"unknown secret field", bytes.Replace(valid, []byte(`,"payload":`), []byte(`,"socket_handle":"private.sock","payload":`), 1)},
		{"tampered provider", bytes.Replace(valid, []byte(`"name":"rust-go"`), []byte(`"name":"evil-go"`), 1)},
		{"tampered team-facing version", bytes.Replace(valid, []byte(`"version":"0.0.1"`), []byte(`"version":"0.0.2"`), 1)},
		{"wrong scope", bytes.Replace(valid, []byte(`"public_ref":"provider_selected"`), []byte(`"public_ref":"other"`), 1)},
		{"wrong capability order", bytes.Replace(valid, []byte(`"capabilities","status":"supported"},{"name":"get_state"`), []byte(`"get_state","status":"supported"},{"name":"capabilities"`), 1)},
		{"missing newline", bytes.TrimSuffix(valid, []byte{'\n'})},
		{"embedded carriage return", append(bytes.TrimSuffix(valid, []byte{'\n'}), '\r', '\n')},
		{"oversize", append(bytes.Repeat([]byte{'x'}, desktopobserve.MaxEnvelopeBytes), '\n')},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _, err := translateDesktopV2Result(binding, test.raw)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.NotContains(t, err.Error(), "private.sock")
		})
	}
}

func TestDesktopV2ResultAdapter_MapsExactTenReasonReceipts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reason    desktopobserve.ReasonCode
		operation desktopobserve.Operation
	}{
		{desktopobserve.ReasonProviderUnavailable, desktopobserve.OperationCapabilities},
		{desktopobserve.ReasonCapabilityUnsupported, desktopobserve.OperationCapabilities},
		{desktopobserve.ReasonAccessibilityPermissionMissing, desktopobserve.OperationPermissions},
		{desktopobserve.ReasonTargetAppNotFound, desktopobserve.OperationListApps},
		{desktopobserve.ReasonTargetWindowNotFound, desktopobserve.OperationListWindows},
		{desktopobserve.ReasonStaleState, desktopobserve.OperationGetState},
		{desktopobserve.ReasonSemanticProjectionUnavailable, desktopobserve.OperationGetState},
		{desktopobserve.ReasonRedactionFailed, desktopobserve.OperationCapabilities},
		{desktopobserve.ReasonEvidenceQuarantined, desktopobserve.OperationGetState},
		{desktopobserve.ReasonProviderProtocolMismatch, desktopobserve.OperationCapabilities},
	}
	for _, test := range tests {
		t.Run(string(test.reason), func(t *testing.T) {
			publicRaw, binding := desktopV2PublicRequest(t, test.operation)
			privateRaw := desktopV2FailureResult(t, binding, test.reason)
			resultRaw, status, err := translateDesktopV2Result(binding, privateRaw)
			require.NoError(t, err)
			assert.Equal(t, desktopV2StatusError, status)
			exchange, err := desktopobserve.DecodeExchange(publicRaw, resultRaw)
			require.NoError(t, err)
			require.NotNil(t, exchange.Result.RuntimeReceipt.ReasonCode)
			assert.Equal(t, test.reason, *exchange.Result.RuntimeReceipt.ReasonCode)
			assert.Equal(t, desktopobserve.NextStep(test.reason), *exchange.Result.RuntimeReceipt.NextStep)
		})
	}

	_, binding := desktopV2PublicRequest(t, desktopobserve.OperationPermissions)
	tampered := bytes.Replace(
		desktopV2FailureResult(t, binding, desktopobserve.ReasonAccessibilityPermissionMissing),
		[]byte("Grant Accessibility access to the signed app and retry explicitly."),
		[]byte("Run an unsafe fallback."), 1,
	)
	result, _, err := translateDesktopV2Result(binding, tampered)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func countPublicDesktopNodesByName(node desktopobserve.SemanticNode, name string) int {
	count := 0
	if node.Name == name {
		count++
	}
	for _, child := range node.Children {
		count += countPublicDesktopNodesByName(child, name)
	}
	return count
}
