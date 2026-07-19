package run

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopEnvelopeTransport_SuccessfulRunnerChainDecodesEveryRawExchange(t *testing.T) {
	t.Parallel()

	transport := newFakeDesktopEnvelopeTransport(t)
	decoder := &countingDesktopExchangeDecoder{}
	client := newEnvelopeDesktopClient(localEnvelopeIdentity(), transport, decoder.Decode)
	outcome, err := newDesktopObservationRunner(
		client, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
	).Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	assert.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
	assert.Equal(t, 5, transport.calls)
	assert.Equal(t, 5, decoder.calls)
	assert.NoError(t, decoder.lastErr)
	assert.Equal(t, []desktopobserve.Operation{
		desktopobserve.OperationCapabilities,
		desktopobserve.OperationPermissions,
		desktopobserve.OperationListApps,
		desktopobserve.OperationListWindows,
		desktopobserve.OperationGetState,
	}, transport.operations)
}

func TestDesktopEnvelopeTransport_MalformedResultIsRejectedByDecoderBeforeLaterOperations(t *testing.T) {
	t.Parallel()

	transport := newFakeDesktopEnvelopeTransport(t)
	transport.malformedOperation = desktopobserve.OperationPermissions
	decoder := &countingDesktopExchangeDecoder{}
	client := newEnvelopeDesktopClient(localEnvelopeIdentity(), transport, decoder.Decode)
	outcome, err := newDesktopObservationRunner(
		client, newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
	).Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	assert.NotEqual(t, desktopobserve.VerdictPassed, outcome.Verdict)
	assert.Nil(t, outcome.SemanticProjection)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, 2, transport.calls)
	assert.Equal(t, 2, decoder.calls)
	assert.ErrorIs(t, decoder.lastErr, desktopobserve.ErrMalformedEnvelope)
	assert.Equal(t, []desktopobserve.Operation{
		desktopobserve.OperationCapabilities,
		desktopobserve.OperationPermissions,
	}, transport.operations)
}

func TestDesktopEnvelopeTransport_FailedReceiptPreservesCurrentCapabilitySummary(t *testing.T) {
	t.Parallel()

	transport := newFakeDesktopEnvelopeTransport(t)
	transport.failedOperation = desktopobserve.OperationGetState
	client := newEnvelopeDesktopClient(localEnvelopeIdentity(), transport, desktopobserve.DecodeExchange)
	alternate := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	outcome, err := newDesktopObservationRunner(client, alternate).Run(
		context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal),
	)
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonCapabilityUnsupported, *outcome.ReasonCode)
	assert.Equal(t, 5, transport.calls)
	assert.Equal(t, desktopobserve.CapabilityUnsupported, capabilityStatus(
		outcome.RuntimeReceipt, desktopobserve.OperationGetState,
	))
	for _, operation := range desktopobserve.ReadOnlyOperations() {
		if operation != desktopobserve.OperationGetState {
			assert.Equal(t, desktopobserve.CapabilitySupported, capabilityStatus(outcome.RuntimeReceipt, operation))
		}
	}
	assert.Empty(t, alternate.calls)
}

func TestDesktopEnvelopeTransport_ContradictoryFailedReceiptIsProtocolMismatch(t *testing.T) {
	t.Parallel()

	transport := newFakeDesktopEnvelopeTransport(t)
	transport.failedOperation = desktopobserve.OperationGetState
	transport.keepFailedCapabilitySupported = true
	client := newEnvelopeDesktopClient(localEnvelopeIdentity(), transport, desktopobserve.DecodeExchange)
	alternate := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	outcome, err := newDesktopObservationRunner(client, alternate).Run(
		context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal),
	)
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonProviderProtocolMismatch, *outcome.ReasonCode)
	assert.Equal(t, 5, transport.calls)
	assert.Empty(t, alternate.calls)
}

func TestDesktopEnvelopeTransport_RoundTripUsesWholeOperationDeadline(t *testing.T) {
	t.Parallel()

	transport := &blockingDesktopEnvelopeTransport{}
	client := newEnvelopeDesktopClient(localEnvelopeIdentity(), transport, desktopobserve.DecodeExchange)
	alternate := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	runner := newDesktopObservationRunner(client, alternate)
	runner.timeout = 10 * time.Millisecond
	started := time.Now()
	outcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonProviderUnavailable, *outcome.ReasonCode)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.Equal(t, 1, transport.calls)
	assert.Empty(t, alternate.calls)
}

type blockingDesktopEnvelopeTransport struct {
	calls int
}

func (transport *blockingDesktopEnvelopeTransport) RoundTrip(ctx context.Context, _ []byte) ([]byte, error) {
	transport.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

type countingDesktopExchangeDecoder struct {
	calls   int
	lastErr error
}

func (decoder *countingDesktopExchangeDecoder) Decode(requestRaw, resultRaw []byte) (desktopobserve.Exchange, error) {
	decoder.calls++
	exchange, err := desktopobserve.DecodeExchange(requestRaw, resultRaw)
	decoder.lastErr = err
	return exchange, err
}

type fakeDesktopEnvelopeTransport struct {
	projection                    desktopobserve.SemanticProjection
	malformedOperation            desktopobserve.Operation
	failedOperation               desktopobserve.Operation
	keepFailedCapabilitySupported bool
	calls                         int
	operations                    []desktopobserve.Operation
}

func newFakeDesktopEnvelopeTransport(t *testing.T) *fakeDesktopEnvelopeTransport {
	t.Helper()
	projection, err := desktopobserve.NormalizeProjection(
		runnerSemanticFixture("provider-local", "state-envelope"),
		func(value string) (string, error) { return value, nil },
	)
	require.NoError(t, err)
	return &fakeDesktopEnvelopeTransport{projection: projection}
}

func (transport *fakeDesktopEnvelopeTransport) RoundTrip(_ context.Context, requestRaw []byte) ([]byte, error) {
	var request struct {
		ProtocolVersion int                         `json:"protocol_version"`
		RequestID       string                      `json:"request_id"`
		Operation       desktopobserve.Operation    `json:"operation"`
		Scope           desktopobserve.ReceiptScope `json:"scope"`
	}
	if err := json.Unmarshal(requestRaw, &request); err != nil {
		return nil, err
	}
	transport.calls++
	transport.operations = append(transport.operations, request.Operation)
	if request.Operation == transport.malformedOperation {
		return []byte(`{"protocol_version":`), nil
	}
	if request.Operation == transport.failedOperation {
		receipt := envelopeReceipt(request.Scope)
		reason := desktopobserve.ReasonCapabilityUnsupported
		nextStep := desktopobserve.NextStep(reason)
		receipt.ReasonCode = &reason
		receipt.NextStep = &nextStep
		receipt.Redaction.Status = desktopobserve.RedactionNotRequired
		receipt.Quarantine.Status = desktopobserve.QuarantineEmpty
		if !transport.keepFailedCapabilitySupported {
			for index := range receipt.CapabilitySummary {
				if receipt.CapabilitySummary[index].Name == request.Operation {
					receipt.CapabilitySummary[index].Status = desktopobserve.CapabilityUnsupported
				}
			}
		}
		return json.Marshal(map[string]any{
			"protocol_version": desktopobserve.ProtocolVersion,
			"request_id":       request.RequestID,
			"status":           "failed",
			"runtime_receipt":  receipt,
		})
	}
	payload := transport.payload(request.Operation)
	return json.Marshal(map[string]any{
		"protocol_version": desktopobserve.ProtocolVersion,
		"request_id":       request.RequestID,
		"status":           "passed",
		"payload":          payload,
		"runtime_receipt":  envelopeReceipt(request.Scope),
	})
}

func (transport *fakeDesktopEnvelopeTransport) payload(operation desktopobserve.Operation) any {
	switch operation {
	case desktopobserve.OperationCapabilities:
		return map[string]any{"capabilities": envelopeCapabilities()}
	case desktopobserve.OperationPermissions:
		return map[string]any{"accessibility_granted": true}
	case desktopobserve.OperationListApps:
		return map[string]any{"apps": []desktopobserve.AppSummary{{AppRef: "autopus-desktop"}}}
	case desktopobserve.OperationListWindows:
		return map[string]any{"windows": []desktopobserve.WindowSummary{{WindowRef: "main-window"}}}
	case desktopobserve.OperationGetState:
		return map[string]any{"semantic_projection": transport.projection}
	default:
		return map[string]any{}
	}
}

func localEnvelopeIdentity() desktopobserve.ProviderIdentity {
	return desktopobserve.ProviderIdentity{
		Name: "autopus-desktop-local", Version: "1.0.0", ProtocolVersion: desktopobserve.ProtocolVersion,
	}
}

func envelopeCapabilities() []desktopobserve.CapabilityStatus {
	capabilities := make([]desktopobserve.CapabilityStatus, 0, len(desktopobserve.ReadOnlyOperations()))
	for _, operation := range desktopobserve.ReadOnlyOperations() {
		capabilities = append(capabilities, desktopobserve.CapabilityStatus{
			Name: operation, Status: desktopobserve.CapabilitySupported,
		})
	}
	return capabilities
}

func envelopeReceipt(scope desktopobserve.ReceiptScope) desktopobserve.RuntimeReceipt {
	return desktopobserve.RuntimeReceipt{
		SchemaVersion:     desktopobserve.RuntimeReceiptSchemaVersion,
		Provider:          localEnvelopeIdentity(),
		Scope:             scope,
		CapabilitySummary: envelopeCapabilities(),
		Redaction:         desktopobserve.RedactionReceipt{Status: desktopobserve.RedactionApplied},
		Quarantine:        desktopobserve.QuarantineReceipt{Status: desktopobserve.QuarantineCleared},
	}
}

func capabilityStatus(
	receipt desktopobserve.RuntimeReceipt,
	operation desktopobserve.Operation,
) desktopobserve.CapabilityState {
	for _, capability := range receipt.CapabilitySummary {
		if capability.Name == operation {
			return capability.Status
		}
	}
	return ""
}
