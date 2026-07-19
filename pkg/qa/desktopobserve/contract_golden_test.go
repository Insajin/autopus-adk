package desktopobserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolV1GoldenExchange_AllFiveOperationsDecodeStrictly(t *testing.T) {
	t.Parallel()

	projection, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	projectionBody, err := json.Marshal(projection)
	require.NoError(t, err)
	tests := []struct {
		operation Operation
		scope     ReceiptScope
		payload   string
	}{
		{OperationCapabilities, ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"}, `{"capabilities":[{"name":"capabilities","status":"supported"},{"name":"get_state","status":"supported"},{"name":"list_apps","status":"supported"},{"name":"list_windows","status":"supported"},{"name":"permissions","status":"supported"}]}`},
		{OperationPermissions, ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"}, `{"accessibility_granted":true}`},
		{OperationListApps, ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"}, `{"apps":[{"app_ref":"autopus-desktop"}]}`},
		{OperationListWindows, ReceiptScope{Kind: ScopeApplication, PublicRef: "autopus-desktop"}, `{"windows":[{"window_ref":"main-window"}]}`},
		{OperationGetState, ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"}, fmt.Sprintf(`{"semantic_projection":%s}`, projectionBody)},
	}

	for index, test := range tests {
		test := test
		t.Run(string(test.operation), func(t *testing.T) {
			t.Parallel()
			requestID := fmt.Sprintf("req-%d", index+1)
			requestRaw := []byte(fmt.Sprintf(
				`{"protocol_version":1,"request_id":%q,"operation":%q,"scope":{"kind":%q,"public_ref":%q}}`,
				requestID, test.operation, test.scope.Kind, test.scope.PublicRef,
			))
			resultRaw := goldenResultEnvelope(t, requestID, test.operation, test.scope, test.payload)
			exchange, err := DecodeExchange(requestRaw, resultRaw)
			require.NoError(t, err)
			assert.Equal(t, test.operation, exchange.Request.Operation)
			assert.Equal(t, "passed", string(exchange.Result.Status))
			assert.JSONEq(t, test.payload, string(exchange.Result.Payload))
			require.NoError(t, exchange.Result.RuntimeReceipt.Validate())
		})
	}
}

func TestProtocolV1GoldenExchange_RejectsMissingNestedDuplicateAndInvalidStatus(t *testing.T) {
	t.Parallel()

	validRequest := `{"protocol_version":1,"request_id":"req-1","operation":"permissions","scope":{"kind":"provider","public_ref":"autopus-desktop-local"}}`
	validResult := string(goldenResultEnvelope(
		t, "req-1", OperationPermissions,
		ReceiptScope{Kind: ScopeProvider, PublicRef: "autopus-desktop-local"},
		`{"accessibility_granted":true}`,
	))
	tests := []struct {
		name       string
		requestRaw string
		resultRaw  string
		want       error
	}{
		{name: "missing request protocol", requestRaw: strings.Replace(validRequest, `"protocol_version":1,`, "", 1), resultRaw: validResult, want: ErrMissingField},
		{name: "missing request id", requestRaw: strings.Replace(validRequest, `"request_id":"req-1",`, "", 1), resultRaw: validResult, want: ErrMissingField},
		{name: "missing operation", requestRaw: strings.Replace(validRequest, `,"operation":"permissions"`, "", 1), resultRaw: validResult, want: ErrMissingField},
		{name: "missing scope", requestRaw: strings.Replace(validRequest, `,"scope":{"kind":"provider","public_ref":"autopus-desktop-local"}`, "", 1), resultRaw: validResult, want: ErrMissingField},
		{name: "missing nested public ref", requestRaw: strings.Replace(validRequest, `,"public_ref":"autopus-desktop-local"`, "", 1), resultRaw: validResult, want: ErrMissingField},
		{name: "nested request unknown", requestRaw: strings.Replace(validRequest, `"public_ref":"autopus-desktop-local"`, `"public_ref":"autopus-desktop-local","pid":42`, 1), resultRaw: validResult, want: ErrUnknownField},
		{name: "nested request duplicate", requestRaw: strings.Replace(validRequest, `"kind":"provider"`, `"kind":"provider","kind":"window"`, 1), resultRaw: validResult, want: ErrDuplicateKey},
		{name: "missing result protocol", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"protocol_version":1,`, "", 1), want: ErrMissingField},
		{name: "missing result request id", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"request_id":"req-1",`, "", 1), want: ErrMissingField},
		{name: "missing status", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"status":"passed",`, "", 1), want: ErrMissingField},
		{name: "invalid status", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"status":"passed"`, `"status":"warning"`, 1), want: ErrInvalidStatus},
		{name: "missing payload", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `,"payload":{"accessibility_granted":true}`, "", 1), want: ErrMissingField},
		{name: "missing runtime receipt", requestRaw: validRequest, resultRaw: `{"protocol_version":1,"request_id":"req-1","status":"passed","payload":{"accessibility_granted":true}}`, want: ErrMissingField},
		{name: "operation payload mismatch", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `{"accessibility_granted":true}`, `{"apps":[{"app_ref":"autopus-desktop"}]}`, 1), want: ErrUnknownField},
		{name: "nested payload unknown", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"accessibility_granted":true`, `"accessibility_granted":true,"raw_action":"press"`, 1), want: ErrUnknownField},
		{name: "nested payload duplicate", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"accessibility_granted":true`, `"accessibility_granted":true,"accessibility_granted":false`, 1), want: ErrDuplicateKey},
		{name: "nested receipt unknown", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"protocol_version":1},"scope"`, `"protocol_version":1,"helper_path":"/tmp/helper"},"scope"`, 1), want: ErrUnknownField},
		{name: "nested receipt duplicate", requestRaw: validRequest, resultRaw: strings.Replace(validResult, `"name":"autopus-desktop-local"`, `"name":"autopus-desktop-local","name":"orca-computer-use-macos"`, 1), want: ErrDuplicateKey},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeExchange([]byte(test.requestRaw), []byte(test.resultRaw))
			require.Error(t, err)
			assert.True(t, errors.Is(err, test.want), "error = %v, want %v", err, test.want)
		})
	}
}

func goldenResultEnvelope(
	t *testing.T,
	requestID string,
	operation Operation,
	scope ReceiptScope,
	payload string,
) []byte {
	t.Helper()
	runtimeReceipt := successfulReceiptForScope(RuntimeProviderLocal, scope)
	if operation == OperationGetState {
		runtimeReceipt.Redaction.Status = RedactionApplied
		runtimeReceipt.Quarantine.Status = QuarantineCleared
	}
	receipt, err := json.Marshal(runtimeReceipt)
	require.NoError(t, err)
	return []byte(fmt.Sprintf(
		`{"protocol_version":1,"request_id":%q,"status":"passed","payload":%s,"runtime_receipt":%s}`,
		requestID, payload, receipt,
	))
}
