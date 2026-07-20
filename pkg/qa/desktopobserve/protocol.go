package desktopobserve

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func ReadOnlyOperations() []Operation {
	return []Operation{
		OperationCapabilities,
		OperationGetState,
		OperationListApps,
		OperationListWindows,
		OperationPermissions,
	}
}

func IsReadOnlyOperation(operation Operation) bool {
	for _, allowed := range ReadOnlyOperations() {
		if operation == allowed {
			return true
		}
	}
	return false
}

func DecodeRequest(raw []byte) (Request, error) {
	var request Request
	if err := decodeStrict(raw, MaxEnvelopeBytes, &request); err != nil {
		return Request{}, err
	}
	keys, err := objectKeys(raw)
	if err != nil {
		return Request{}, err
	}
	if !hasRequiredKeys(keys, "protocol_version", "request_id", "operation", "scope") {
		return Request{}, fmt.Errorf("%w: request", ErrMissingField)
	}
	if request.ProtocolVersion != ProtocolVersion {
		return Request{}, ErrProtocolMismatch
	}
	if request.RequestID == "" {
		return Request{}, fmt.Errorf("%w: request_id", ErrMissingField)
	}
	if request.Operation == "" {
		return Request{}, fmt.Errorf("%w: operation", ErrMissingField)
	}
	if !IsReadOnlyOperation(request.Operation) {
		return Request{}, ErrUnsupportedOperation
	}
	if request.Scope.Kind == "" || request.Scope.PublicRef == "" {
		return Request{}, fmt.Errorf("%w: scope", ErrMissingField)
	}
	if !validRequestScope(request.Operation, request.Scope) {
		return Request{}, fmt.Errorf("%w: scope", ErrMalformedEnvelope)
	}
	if expected, present := keys["expected_state_ref"]; present {
		if bytes.Equal(bytes.TrimSpace(expected), []byte("null")) ||
			!validOpaqueRef(request.ExpectedStateRef, "state-") {
			return Request{}, fmt.Errorf("%w: expected_state_ref", ErrMalformedEnvelope)
		}
		if request.Operation != OperationGetState {
			return Request{}, ErrUnsupportedOperation
		}
	}
	return request, nil
}

func DecodeResult(raw []byte, requestID string) (Result, error) {
	type resultWire struct {
		ProtocolVersion int             `json:"protocol_version"`
		RequestID       string          `json:"request_id"`
		Status          ResultStatus    `json:"status"`
		Payload         json.RawMessage `json:"payload,omitempty"`
		RuntimeReceipt  json.RawMessage `json:"runtime_receipt"`
	}
	var wire resultWire
	if err := decodeStrict(raw, MaxEnvelopeBytes, &wire); err != nil {
		return Result{}, err
	}
	keys, err := objectKeys(raw)
	if err != nil {
		return Result{}, err
	}
	if !hasRequiredKeys(keys, "protocol_version", "request_id", "status", "runtime_receipt") {
		return Result{}, fmt.Errorf("%w: result", ErrMissingField)
	}
	if wire.ProtocolVersion != ProtocolVersion {
		return Result{}, ErrProtocolMismatch
	}
	if wire.RequestID == "" {
		return Result{}, fmt.Errorf("%w: request_id", ErrMissingField)
	}
	if wire.RequestID != requestID {
		return Result{}, ErrRequestIDMismatch
	}
	if wire.Status != ResultPassed && wire.Status != ResultFailed {
		return Result{}, ErrInvalidStatus
	}
	if wire.Status == ResultPassed && len(wire.Payload) == 0 {
		return Result{}, fmt.Errorf("%w: payload", ErrMissingField)
	}
	if wire.Status == ResultFailed && len(wire.Payload) != 0 {
		return Result{}, fmt.Errorf("%w: failed payload", ErrUnknownField)
	}
	if len(wire.RuntimeReceipt) == 0 || bytes.Equal(bytes.TrimSpace(wire.RuntimeReceipt), []byte("null")) {
		return Result{}, fmt.Errorf("%w: runtime_receipt", ErrMissingField)
	}
	receipt, err := DecodeRuntimeReceipt(wire.RuntimeReceipt)
	if err != nil {
		return Result{}, err
	}
	if (wire.Status == ResultPassed) != (receipt.ReasonCode == nil) {
		return Result{}, fmt.Errorf("%w: result receipt status", ErrMalformedEnvelope)
	}
	return Result{
		ProtocolVersion: wire.ProtocolVersion,
		RequestID:       wire.RequestID,
		Status:          wire.Status,
		Payload:         wire.Payload,
		RuntimeReceipt:  receipt,
	}, nil
}

func DecodeExchange(requestRaw, resultRaw []byte) (Exchange, error) {
	request, err := DecodeRequest(requestRaw)
	if err != nil {
		return Exchange{}, err
	}
	result, err := DecodeResult(resultRaw, request.RequestID)
	if err != nil {
		return Exchange{}, err
	}
	if result.Status == ResultPassed {
		projection, err := validateOperationPayload(request.Operation, result.Payload)
		if err != nil {
			return Exchange{}, err
		}
		if err := validateSuccessBinding(request, result, projection); err != nil {
			return Exchange{}, err
		}
	}
	return Exchange{Request: request, Result: result}, nil
}

func validRequestScope(operation Operation, scope ReceiptScope) bool {
	expected := ScopeProvider
	switch operation {
	case OperationListWindows:
		return scope.Kind == ScopeApplication && scope.PublicRef == "autopus-desktop"
	case OperationGetState:
		return scope.Kind == ScopeWindow && scope.PublicRef == "main-window"
	}
	return scope.Kind == expected &&
		(scope.PublicRef == "autopus-desktop-local" || scope.PublicRef == "orca-computer-use-macos")
}

func validateOperationPayload(operation Operation, raw json.RawMessage) (SemanticProjection, error) {
	switch operation {
	case OperationCapabilities:
		var payload struct {
			Capabilities []CapabilityStatus `json:"capabilities"`
		}
		if err := decodeStrict(raw, MaxEnvelopeBytes, &payload); err != nil {
			return SemanticProjection{}, err
		}
		if !validCapabilities(payload.Capabilities) {
			return SemanticProjection{}, fmt.Errorf("%w: capabilities", ErrMalformedEnvelope)
		}
	case OperationPermissions:
		var payload struct {
			AccessibilityGranted *bool `json:"accessibility_granted"`
		}
		if err := decodeStrict(raw, MaxEnvelopeBytes, &payload); err != nil {
			return SemanticProjection{}, err
		}
		if payload.AccessibilityGranted == nil || !*payload.AccessibilityGranted {
			return SemanticProjection{}, fmt.Errorf("%w: accessibility_granted", ErrMalformedEnvelope)
		}
	case OperationListApps:
		var payload struct {
			Apps []AppSummary `json:"apps"`
		}
		if err := decodeStrict(raw, MaxEnvelopeBytes, &payload); err != nil {
			return SemanticProjection{}, err
		}
		if !validApps(payload.Apps) {
			return SemanticProjection{}, fmt.Errorf("%w: apps", ErrMalformedEnvelope)
		}
	case OperationListWindows:
		var payload struct {
			Windows []WindowSummary `json:"windows"`
		}
		if err := decodeStrict(raw, MaxEnvelopeBytes, &payload); err != nil {
			return SemanticProjection{}, err
		}
		if !validWindows(payload.Windows) {
			return SemanticProjection{}, fmt.Errorf("%w: windows", ErrMalformedEnvelope)
		}
	case OperationGetState:
		var payload struct {
			SemanticProjection *SemanticProjection `json:"semantic_projection"`
		}
		if err := decodeStrict(raw, MaxEnvelopeBytes, &payload); err != nil {
			return SemanticProjection{}, err
		}
		if payload.SemanticProjection == nil || validateProjection(*payload.SemanticProjection) != nil {
			return SemanticProjection{}, fmt.Errorf("%w: semantic_projection", ErrMalformedEnvelope)
		}
		return *payload.SemanticProjection, nil
	default:
		return SemanticProjection{}, ErrUnsupportedOperation
	}
	return SemanticProjection{}, nil
}

func validApps(apps []AppSummary) bool {
	return len(apps) == 1 && apps[0].AppRef == "autopus-desktop"
}

func validWindows(windows []WindowSummary) bool {
	return len(windows) == 1 && windows[0].WindowRef == "main-window"
}

func validateSuccessBinding(request Request, result Result, projection SemanticProjection) error {
	receipt := result.RuntimeReceipt
	switch request.Operation {
	case OperationCapabilities, OperationPermissions, OperationListApps:
		if request.Scope.Kind != ScopeProvider || request.Scope.PublicRef != receipt.Provider.Name ||
			receipt.Scope != request.Scope {
			return ErrScopeMismatch
		}
	case OperationListWindows:
		if request.Scope.Kind != ScopeApplication || request.Scope.PublicRef != "autopus-desktop" ||
			receipt.Scope != request.Scope {
			return ErrScopeMismatch
		}
	case OperationGetState:
		if err := validateStateReceipt(receipt); err != nil {
			return err
		}
		if projection.ProviderRef != providerRef(receipt.Provider.Name) ||
			projection.AppRef != "autopus-desktop" || projection.WindowRef != request.Scope.PublicRef ||
			!validOpaqueRef(projection.StateRef, "state-") {
			return ErrScopeMismatch
		}
		windowScope := receipt.Scope == request.Scope
		stateScope := receipt.Scope.Kind == ScopeState && receipt.Scope.PublicRef == projection.StateRef
		if !windowScope && !stateScope {
			return ErrScopeMismatch
		}
	default:
		return ErrUnsupportedOperation
	}
	return nil
}

func providerRef(providerName string) string {
	switch providerName {
	case "autopus-desktop-local":
		return "provider-local"
	case "orca-computer-use-macos":
		return "provider-orca"
	default:
		return ""
	}
}

func safePublicRef(value string) bool {
	return len(value) <= 96 && safePublicRefPattern.MatchString(value)
}
