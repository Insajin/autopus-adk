package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type desktopEnvelopeTransport interface {
	RoundTrip(context.Context, []byte) ([]byte, error)
}

type desktopExchangeDecoder func([]byte, []byte) (desktopobserve.Exchange, error)

type envelopeDesktopClient struct {
	identity                 desktopobserve.ProviderIdentity
	transport                desktopEnvelopeTransport
	decode                   desktopExchangeDecoder
	sequence                 atomic.Uint64
	handshakeViaCapabilities bool
	handshakeDone            bool
	cachedCapabilities       []desktopobserve.Operation
}

func newEnvelopeDesktopClient(
	identity desktopobserve.ProviderIdentity,
	transport desktopEnvelopeTransport,
	decode desktopExchangeDecoder,
) *envelopeDesktopClient {
	return &envelopeDesktopClient{identity: identity, transport: transport, decode: decode}
}

func (client *envelopeDesktopClient) Handshake(ctx context.Context) (desktopobserve.ProviderIdentity, error) {
	if client.handshakeViaCapabilities && !client.handshakeDone {
		exchange, err := client.exchange(
			ctx, desktopobserve.OperationCapabilities, desktopProviderScope(client.identity),
		)
		if err != nil {
			return desktopobserve.ProviderIdentity{}, err
		}
		operations, err := desktopCapabilitiesFromExchange(exchange)
		if err != nil {
			return desktopobserve.ProviderIdentity{}, err
		}
		client.identity = exchange.Result.RuntimeReceipt.Provider
		client.cachedCapabilities = operations
		client.handshakeDone = true
	}
	return client.identity, nil
}

func (client *envelopeDesktopClient) Capabilities(ctx context.Context) ([]desktopobserve.Operation, error) {
	if client.handshakeViaCapabilities && client.handshakeDone {
		return append([]desktopobserve.Operation(nil), client.cachedCapabilities...), nil
	}
	exchange, err := client.exchange(ctx, desktopobserve.OperationCapabilities, desktopProviderScope(client.identity))
	if err != nil {
		return nil, err
	}
	return desktopCapabilitiesFromExchange(exchange)
}

func desktopCapabilitiesFromExchange(exchange desktopobserve.Exchange) ([]desktopobserve.Operation, error) {
	var payload struct {
		Capabilities []desktopobserve.CapabilityStatus `json:"capabilities"`
	}
	if err := json.Unmarshal(exchange.Result.Payload, &payload); err != nil {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	operations := make([]desktopobserve.Operation, 0, len(payload.Capabilities))
	for _, capability := range payload.Capabilities {
		if capability.Status == desktopobserve.CapabilitySupported {
			operations = append(operations, capability.Name)
		}
	}
	return operations, nil
}

func (client *envelopeDesktopClient) Permissions(ctx context.Context) (desktopobserve.PermissionResult, error) {
	exchange, err := client.exchange(ctx, desktopobserve.OperationPermissions, desktopProviderScope(client.identity))
	if err != nil {
		return desktopobserve.PermissionResult{}, err
	}
	var payload desktopobserve.PermissionResult
	if err := json.Unmarshal(exchange.Result.Payload, &payload); err != nil {
		return desktopobserve.PermissionResult{}, desktopobserve.ErrMalformedEnvelope
	}
	return payload, nil
}

func (client *envelopeDesktopClient) ListApps(ctx context.Context) ([]desktopobserve.AppSummary, error) {
	exchange, err := client.exchange(ctx, desktopobserve.OperationListApps, desktopProviderScope(client.identity))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Apps []desktopobserve.AppSummary `json:"apps"`
	}
	if err := json.Unmarshal(exchange.Result.Payload, &payload); err != nil {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	return payload.Apps, nil
}

func (client *envelopeDesktopClient) ListWindows(
	ctx context.Context,
	appRef string,
) ([]desktopobserve.WindowSummary, error) {
	exchange, err := client.exchange(ctx, desktopobserve.OperationListWindows, desktopobserve.ReceiptScope{
		Kind: desktopobserve.ScopeApplication, PublicRef: appRef,
	})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Windows []desktopobserve.WindowSummary `json:"windows"`
	}
	if err := json.Unmarshal(exchange.Result.Payload, &payload); err != nil {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	return payload.Windows, nil
}

func (client *envelopeDesktopClient) GetState(
	ctx context.Context,
	_ string,
	windowRef string,
) (desktopobserve.SemanticProjection, error) {
	exchange, err := client.exchange(ctx, desktopobserve.OperationGetState, desktopWindowScope(windowRef))
	if err != nil {
		return desktopobserve.SemanticProjection{}, err
	}
	var payload struct {
		SemanticProjection desktopobserve.SemanticProjection `json:"semantic_projection"`
	}
	if err := json.Unmarshal(exchange.Result.Payload, &payload); err != nil {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	return payload.SemanticProjection, nil
}

func (client *envelopeDesktopClient) exchange(
	ctx context.Context,
	operation desktopobserve.Operation,
	scope desktopobserve.ReceiptScope,
) (desktopobserve.Exchange, error) {
	if client == nil || client.transport == nil || client.decode == nil {
		return desktopobserve.Exchange{}, errors.New("desktop envelope client is unavailable")
	}
	request := desktopobserve.Request{
		ProtocolVersion: desktopobserve.ProtocolVersion,
		RequestID:       fmt.Sprintf("desktop-observe-%d", client.sequence.Add(1)),
		Operation:       operation,
		Scope:           scope,
	}
	requestRaw, err := json.Marshal(request)
	if err != nil {
		return desktopobserve.Exchange{}, desktopobserve.ErrMalformedEnvelope
	}
	requestRaw = append(requestRaw, '\n')
	if len(requestRaw) > desktopobserve.MaxEnvelopeBytes {
		return desktopobserve.Exchange{}, desktopobserve.ErrEnvelopeTooLarge
	}
	resultRaw, err := client.transport.RoundTrip(ctx, requestRaw)
	if err != nil {
		return desktopobserve.Exchange{}, err
	}
	exchange, err := client.decode(requestRaw, resultRaw)
	if err != nil {
		return desktopobserve.Exchange{}, err
	}
	if exchange.Result.Status == desktopobserve.ResultFailed {
		if exchange.Result.RuntimeReceipt.ReasonCode == nil {
			return desktopobserve.Exchange{}, desktopobserve.ErrMalformedEnvelope
		}
		allowProviderVersion := client.handshakeViaCapabilities && !client.handshakeDone
		if !validFailedDesktopReceipt(
			client.identity, request, exchange.Result.RuntimeReceipt, allowProviderVersion,
		) {
			return desktopobserve.Exchange{}, desktopobserve.ErrMalformedEnvelope
		}
		return desktopobserve.Exchange{}, desktopFailureForReason(
			*exchange.Result.RuntimeReceipt.ReasonCode,
			operation,
			exchange.Result.RuntimeReceipt,
		)
	}
	return exchange, nil
}

func validFailedDesktopReceipt(
	identity desktopobserve.ProviderIdentity,
	request desktopobserve.Request,
	receipt desktopobserve.RuntimeReceipt,
	allowProviderVersion bool,
) bool {
	providerMatches := receipt.Provider == identity
	if allowProviderVersion {
		providerMatches = receipt.Provider.Name == identity.Name &&
			receipt.Provider.ProtocolVersion == identity.ProtocolVersion
	}
	if !providerMatches || receipt.Scope != request.Scope || receipt.ReasonCode == nil {
		return false
	}
	condition, ok := desktopFailureConditionForReason(*receipt.ReasonCode)
	if !ok || !desktopFailureReasonAllowed(*receipt.ReasonCode, request.Operation) {
		return false
	}
	if condition != desktopobserve.FailureOperationMissing {
		return true
	}
	for _, capability := range receipt.CapabilitySummary {
		if capability.Name == request.Operation {
			return capability.Status == desktopobserve.CapabilityUnsupported
		}
	}
	return false
}

func desktopFailureReasonAllowed(
	reason desktopobserve.ReasonCode,
	operation desktopobserve.Operation,
) bool {
	switch reason {
	case desktopobserve.ReasonProviderUnavailable,
		desktopobserve.ReasonCapabilityUnsupported,
		desktopobserve.ReasonProviderProtocolMismatch,
		desktopobserve.ReasonRedactionFailed:
		return desktopobserve.IsReadOnlyOperation(operation)
	case desktopobserve.ReasonAccessibilityPermissionMissing:
		return operation != desktopobserve.OperationCapabilities
	case desktopobserve.ReasonTargetAppNotFound:
		return operation == desktopobserve.OperationListApps ||
			operation == desktopobserve.OperationListWindows || operation == desktopobserve.OperationGetState
	case desktopobserve.ReasonTargetWindowNotFound:
		return operation == desktopobserve.OperationListWindows || operation == desktopobserve.OperationGetState
	case desktopobserve.ReasonStaleState,
		desktopobserve.ReasonSemanticProjectionUnavailable,
		desktopobserve.ReasonEvidenceQuarantined:
		return operation == desktopobserve.OperationGetState
	default:
		return false
	}
}
