package desktopobserve

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOnlyOperationAllowlist_ExactFiveSorted(t *testing.T) {
	t.Parallel()

	want := []Operation{
		OperationCapabilities,
		OperationGetState,
		OperationListApps,
		OperationListWindows,
		OperationPermissions,
	}

	assert.Equal(t, want, ReadOnlyOperations())
	assert.True(t, IsReadOnlyOperation(OperationGetState))
	for _, operation := range []Operation{"click", "screenshot", "raw_tree", "shell", "file"} {
		assert.False(t, IsReadOnlyOperation(operation), operation)
	}
}

func TestDecodeRequest_StrictEnvelope(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"protocol_version":1,
		"request_id":"req-001",
		"operation":"get_state",
		"scope":{"kind":"window","public_ref":"main-window"},
		"expected_state_ref":"state-001"
	}`)

	request, err := DecodeRequest(raw)
	require.NoError(t, err)
	assert.Equal(t, ProtocolVersion, request.ProtocolVersion)
	assert.Equal(t, "req-001", request.RequestID)
	assert.Equal(t, OperationGetState, request.Operation)
	assert.Equal(t, ScopeWindow, request.Scope.Kind)
	assert.Equal(t, "main-window", request.Scope.PublicRef)
	assert.Equal(t, "state-001", request.ExpectedStateRef)
}

func TestDecodeRequest_RejectsUnsafeOrMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{
			name: "unknown field",
			raw:  []byte(`{"protocol_version":1,"request_id":"r","operation":"permissions","scope":{"kind":"provider","public_ref":"local"},"screenshot":true}`),
			want: ErrUnknownField,
		},
		{
			name: "duplicate key",
			raw:  []byte(`{"protocol_version":1,"request_id":"r","request_id":"other","operation":"permissions","scope":{"kind":"provider","public_ref":"local"}}`),
			want: ErrDuplicateKey,
		},
		{
			name: "unsupported operation",
			raw:  []byte(`{"protocol_version":1,"request_id":"r","operation":"click","scope":{"kind":"window","public_ref":"main-window"}}`),
			want: ErrUnsupportedOperation,
		},
		{
			name: "protocol mismatch",
			raw:  []byte(`{"protocol_version":2,"request_id":"r","operation":"permissions","scope":{"kind":"provider","public_ref":"local"}}`),
			want: ErrProtocolMismatch,
		},
		{
			name: "malformed json",
			raw:  []byte(`{"protocol_version":1`),
			want: ErrMalformedEnvelope,
		},
		{
			name: "malformed utf8",
			raw:  append([]byte(`{"protocol_version":1,"request_id":"`), 0xff),
			want: ErrMalformedEnvelope,
		},
		{
			name: "oversized",
			raw:  bytes.Repeat([]byte("x"), MaxEnvelopeBytes+1),
			want: ErrEnvelopeTooLarge,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeRequest(test.raw)
			require.Error(t, err)
			assert.True(t, errors.Is(err, test.want), "error = %v, want %v", err, test.want)
		})
	}
}

func TestDecodeResult_ValidatesProtocolAndRequestBeforePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		requestID string
		want      error
	}{
		{
			name:      "protocol mismatch",
			raw:       `{"protocol_version":2,"request_id":"req-1","status":"failed","runtime_receipt":{}}`,
			requestID: "req-1",
			want:      ErrProtocolMismatch,
		},
		{
			name:      "request id mismatch",
			raw:       `{"protocol_version":1,"request_id":"req-2","status":"failed","runtime_receipt":{}}`,
			requestID: "req-1",
			want:      ErrRequestIDMismatch,
		},
		{
			name:      "unknown field",
			raw:       `{"protocol_version":1,"request_id":"req-1","status":"failed","runtime_receipt":{},"raw_tree":{}}`,
			requestID: "req-1",
			want:      ErrUnknownField,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeResult([]byte(test.raw), test.requestID)
			require.Error(t, err)
			assert.True(t, errors.Is(err, test.want), "error = %v, want %v", err, test.want)
		})
	}
}

func TestDecodeResult_RejectsDuplicateOversizedAndMalformedEnvelopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{
			name: "duplicate key",
			raw:  []byte(`{"protocol_version":1,"request_id":"req-1","request_id":"req-2","status":"failed","runtime_receipt":{}}`),
			want: ErrDuplicateKey,
		},
		{
			name: "oversized",
			raw:  bytes.Repeat([]byte("x"), MaxEnvelopeBytes+1),
			want: ErrEnvelopeTooLarge,
		},
		{
			name: "malformed utf8",
			raw:  append([]byte(`{"protocol_version":1,"request_id":"`), 0xff),
			want: ErrMalformedEnvelope,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeResult(test.raw, "req-1")
			require.Error(t, err)
			assert.True(t, errors.Is(err, test.want), "error = %v, want %v", err, test.want)
		})
	}
}
