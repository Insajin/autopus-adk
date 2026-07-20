package run

import (
	"errors"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type desktopProviderFailure struct {
	condition desktopobserve.FailureCondition
	operation desktopobserve.Operation
	receipt   desktopobserve.RuntimeReceipt
	validated bool
}

func (failure desktopProviderFailure) Error() string {
	return "desktop provider returned a normalized failure"
}

func desktopFailureOutcome(
	condition desktopobserve.FailureCondition,
	provider desktopobserve.ProviderIdentity,
	scope desktopobserve.ReceiptScope,
	capabilities []desktopobserve.CapabilityStatus,
	operation desktopobserve.Operation,
) (desktopobserve.OracleOutcome, error) {
	if condition == desktopobserve.FailureOperationMissing {
		if !desktopobserve.IsReadOnlyOperation(operation) {
			operation = desktopobserve.OperationCapabilities
		}
		capabilities = desktopMarkUnsupported(capabilities, operation)
	}
	return desktopobserve.NormalizeFailure(desktopobserve.FailureSignal{
		Condition:          condition,
		Provider:           provider,
		Scope:              scope,
		CapabilitySummary:  capabilities,
		RequestedOperation: operation,
	})
}

func desktopFailureFromError(
	err error,
	provider desktopobserve.ProviderIdentity,
	scope desktopobserve.ReceiptScope,
	capabilities []desktopobserve.CapabilityStatus,
) (desktopobserve.OracleOutcome, error) {
	var providerFailure desktopProviderFailure
	if errors.As(err, &providerFailure) && providerFailure.validated && providerFailure.receipt.Validate() == nil {
		return desktopFailureOutcome(
			providerFailure.condition,
			providerFailure.receipt.Provider,
			providerFailure.receipt.Scope,
			providerFailure.receipt.CapabilitySummary,
			providerFailure.operation,
		)
	}
	condition, operation := desktopFailureCondition(err)
	return desktopFailureOutcome(condition, provider, scope, capabilities, operation)
}

func desktopFailureCondition(err error) (desktopobserve.FailureCondition, desktopobserve.Operation) {
	var providerFailure desktopProviderFailure
	if errors.As(err, &providerFailure) {
		return providerFailure.condition, providerFailure.operation
	}
	switch {
	case errors.Is(err, desktopobserve.ErrRawOnlyEvidence):
		return desktopobserve.FailureRawOnlyQuarantine, ""
	case errors.Is(err, desktopobserve.ErrRedactionFailed):
		return desktopobserve.FailureRedaction, ""
	case desktopProtocolError(err):
		return desktopobserve.FailureProtocolVersion, ""
	default:
		return desktopobserve.FailureProviderStart, ""
	}
}

func desktopProtocolError(err error) bool {
	return errors.Is(err, desktopobserve.ErrDuplicateKey) ||
		errors.Is(err, desktopobserve.ErrEnvelopeTooLarge) ||
		errors.Is(err, desktopobserve.ErrInvalidStatus) ||
		errors.Is(err, desktopobserve.ErrMalformedEnvelope) ||
		errors.Is(err, desktopobserve.ErrMissingField) ||
		errors.Is(err, desktopobserve.ErrProtocolMismatch) ||
		errors.Is(err, desktopobserve.ErrRequestIDMismatch) ||
		errors.Is(err, desktopobserve.ErrScopeMismatch) ||
		errors.Is(err, desktopobserve.ErrUnknownField) ||
		errors.Is(err, desktopobserve.ErrUnsupportedOperation)
}

func desktopFailureForReason(
	reason desktopobserve.ReasonCode,
	operation desktopobserve.Operation,
	receipt desktopobserve.RuntimeReceipt,
) desktopProviderFailure {
	condition, _ := desktopFailureConditionForReason(reason)
	return desktopProviderFailure{
		condition: condition, operation: operation, receipt: receipt, validated: true,
	}
}

func desktopFailureConditionForReason(
	reason desktopobserve.ReasonCode,
) (desktopobserve.FailureCondition, bool) {
	condition, ok := map[desktopobserve.ReasonCode]desktopobserve.FailureCondition{
		desktopobserve.ReasonProviderUnavailable:            desktopobserve.FailureProviderStart,
		desktopobserve.ReasonCapabilityUnsupported:          desktopobserve.FailureOperationMissing,
		desktopobserve.ReasonAccessibilityPermissionMissing: desktopobserve.FailureAccessibilityDenied,
		desktopobserve.ReasonTargetAppNotFound:              desktopobserve.FailureAppAliasUnmatched,
		desktopobserve.ReasonTargetWindowNotFound:           desktopobserve.FailureWindowAliasUnmatched,
		desktopobserve.ReasonStaleState:                     desktopobserve.FailureStateRefRejected,
		desktopobserve.ReasonSemanticProjectionUnavailable:  desktopobserve.FailureLandmarksInsufficient,
		desktopobserve.ReasonRedactionFailed:                desktopobserve.FailureRedaction,
		desktopobserve.ReasonEvidenceQuarantined:            desktopobserve.FailureRawOnlyQuarantine,
		desktopobserve.ReasonProviderProtocolMismatch:       desktopobserve.FailureProtocolVersion,
	}[reason]
	return condition, ok
}

func desktopCapabilitySummary(operations []desktopobserve.Operation) []desktopobserve.CapabilityStatus {
	supported := make(map[desktopobserve.Operation]bool, len(operations))
	for _, operation := range operations {
		if desktopobserve.IsReadOnlyOperation(operation) {
			supported[operation] = true
		}
	}
	result := make([]desktopobserve.CapabilityStatus, 0, len(desktopobserve.ReadOnlyOperations()))
	for _, operation := range desktopobserve.ReadOnlyOperations() {
		status := desktopobserve.CapabilityUnsupported
		if supported[operation] {
			status = desktopobserve.CapabilitySupported
		}
		result = append(result, desktopobserve.CapabilityStatus{Name: operation, Status: status})
	}
	return result
}

func desktopSupportedCapabilities() []desktopobserve.CapabilityStatus {
	return desktopCapabilitySummary(desktopobserve.ReadOnlyOperations())
}

func desktopUnsupportedCapabilities() []desktopobserve.CapabilityStatus {
	return desktopCapabilitySummary(nil)
}

func desktopMarkUnsupported(
	capabilities []desktopobserve.CapabilityStatus,
	operation desktopobserve.Operation,
) []desktopobserve.CapabilityStatus {
	copy := append([]desktopobserve.CapabilityStatus(nil), capabilities...)
	if len(copy) != len(desktopobserve.ReadOnlyOperations()) {
		copy = desktopSupportedCapabilities()
	}
	for index := range copy {
		if copy[index].Name == operation {
			copy[index].Status = desktopobserve.CapabilityUnsupported
		}
	}
	return copy
}

func desktopFirstMissingCapability(capabilities []desktopobserve.CapabilityStatus) desktopobserve.Operation {
	for _, requested := range desktopExecutionOperations() {
		for _, capability := range capabilities {
			if capability.Name == requested && capability.Status == desktopobserve.CapabilityUnsupported {
				return requested
			}
		}
	}
	return ""
}

func desktopSuccessReceipt(
	provider desktopobserve.ProviderIdentity,
	scope desktopobserve.ReceiptScope,
	capabilities []desktopobserve.CapabilityStatus,
) desktopobserve.RuntimeReceipt {
	return desktopobserve.RuntimeReceipt{
		SchemaVersion:     desktopobserve.RuntimeReceiptSchemaVersion,
		Provider:          provider,
		Scope:             scope,
		CapabilitySummary: append([]desktopobserve.CapabilityStatus(nil), capabilities...),
		Redaction:         desktopobserve.RedactionReceipt{Status: desktopobserve.RedactionApplied},
		Quarantine:        desktopobserve.QuarantineReceipt{Status: desktopobserve.QuarantineCleared},
	}
}
