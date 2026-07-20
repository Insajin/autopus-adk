package run

import (
	"bytes"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

type desktopV2Provider struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
}

type desktopV2Capability struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type desktopV2Status struct {
	Status string `json:"status"`
}

type desktopV2Receipt struct {
	SchemaVersion     string                `json:"schema_version"`
	Provider          desktopV2Provider     `json:"provider"`
	Scope             desktopV2Scope        `json:"scope"`
	CapabilitySummary []desktopV2Capability `json:"capability_summary"`
	ReasonCode        *string               `json:"reason_code"`
	NextStep          *string               `json:"next_step"`
	Redaction         desktopV2Status       `json:"redaction"`
	Quarantine        desktopV2Status       `json:"quarantine"`
}

var desktopV2NextSteps = map[desktopobserve.ReasonCode]string{
	desktopobserve.ReasonAccessibilityPermissionMissing: "Grant Accessibility access to the signed app and retry explicitly.",
	desktopobserve.ReasonTargetAppNotFound:              "Launch exactly one signed Autopus Desktop target and retry.",
	desktopobserve.ReasonTargetWindowNotFound:           "Open and focus the public main window, then retry.",
	desktopobserve.ReasonStaleState:                     "Request a fresh state without reusing an earlier state reference.",
	desktopobserve.ReasonSemanticProjectionUnavailable:  "Expose the required application and window landmarks, then retry.",
	desktopobserve.ReasonCapabilityUnsupported:          "Use one of the five advertised read-only operations.",
	desktopobserve.ReasonProviderUnavailable:            "Retry with the exact bounded provider contract.",
	desktopobserve.ReasonRedactionFailed:                "Retry with the exact bounded provider contract.",
	desktopobserve.ReasonEvidenceQuarantined:            "Retry with the exact bounded provider contract.",
	desktopobserve.ReasonProviderProtocolMismatch:       "Retry with the exact bounded provider contract.",
}

func decodeDesktopV2Receipt(
	raw []byte,
	binding desktopV2Binding,
	resultStatus string,
) (desktopV2Receipt, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return desktopV2Receipt{}, desktopobserve.ErrMissingField
	}
	var receipt desktopV2Receipt
	if err := decodeDesktopV2Object(
		raw, &receipt, "schema_version", "provider", "scope", "capability_summary",
		"reason_code", "next_step", "redaction", "quarantine",
	); err != nil {
		return desktopV2Receipt{}, err
	}
	if receipt.SchemaVersion != desktopobserve.RuntimeReceiptSchemaVersion ||
		receipt.Provider != (desktopV2Provider{Name: "rust-go", Version: "0.0.1", ProtocolVersion: 2}) ||
		receipt.Scope != binding.privateRequest.Scope || receipt.Quarantine.Status != "empty" ||
		!validDesktopV2Capabilities(receipt.CapabilitySummary) {
		return desktopV2Receipt{}, desktopobserve.ErrMalformedEnvelope
	}
	if resultStatus == desktopV2StatusOK {
		expectedRedaction := "not_required"
		if binding.publicRequest.Operation == desktopobserve.OperationGetState {
			expectedRedaction = "applied"
		}
		if receipt.ReasonCode != nil || receipt.NextStep != nil || receipt.Redaction.Status != expectedRedaction {
			return desktopV2Receipt{}, desktopobserve.ErrMalformedEnvelope
		}
		return receipt, nil
	}
	if receipt.ReasonCode == nil || receipt.NextStep == nil || receipt.Redaction.Status != "not_required" {
		return desktopV2Receipt{}, desktopobserve.ErrMalformedEnvelope
	}
	reason := desktopobserve.ReasonCode(*receipt.ReasonCode)
	if expected, ok := desktopV2NextSteps[reason]; !ok || *receipt.NextStep != expected ||
		!desktopFailureReasonAllowed(reason, binding.publicRequest.Operation) {
		return desktopV2Receipt{}, desktopobserve.ErrMalformedEnvelope
	}
	return receipt, nil
}

func validDesktopV2Capabilities(capabilities []desktopV2Capability) bool {
	if len(capabilities) != len(desktopV2Operations) {
		return false
	}
	for index, capability := range capabilities {
		if capability.Name != desktopV2Operations[index] || capability.Status != "supported" {
			return false
		}
	}
	return true
}

func mapDesktopV2Result(
	binding desktopV2Binding,
	wire desktopV2ResultWire,
	receipt desktopV2Receipt,
) (desktopobserve.Result, error) {
	publicReceipt := desktopSuccessReceipt(
		desktopobserve.ProviderIdentity{Name: "autopus-desktop-local", Version: "0.0.1", ProtocolVersion: 1},
		binding.publicRequest.Scope,
		desktopSupportedCapabilities(),
	)
	publicReceipt.Redaction.Status = desktopobserve.RedactionNotRequired
	publicReceipt.Quarantine.Status = desktopobserve.QuarantineEmpty
	result := desktopobserve.Result{
		ProtocolVersion: desktopobserve.ProtocolVersion,
		RequestID:       binding.publicRequest.RequestID,
		Status:          desktopobserve.ResultPassed,
		RuntimeReceipt:  publicReceipt,
	}
	if wire.Status == desktopV2StatusOK {
		payload, err := mapDesktopV2Payload(binding.publicRequest.Operation, wire.Payload)
		if err != nil {
			return desktopobserve.Result{}, err
		}
		result.Payload = payload
		if binding.publicRequest.Operation == desktopobserve.OperationGetState {
			result.RuntimeReceipt.Redaction.Status = desktopobserve.RedactionApplied
			result.RuntimeReceipt.Quarantine.Status = desktopobserve.QuarantineCleared
		}
		return result, nil
	}
	reason := desktopobserve.ReasonCode(*receipt.ReasonCode)
	nextStep := desktopobserve.NextStep(reason)
	result.Status = desktopobserve.ResultFailed
	result.RuntimeReceipt.ReasonCode = &reason
	result.RuntimeReceipt.NextStep = &nextStep
	if reason == desktopobserve.ReasonCapabilityUnsupported {
		result.RuntimeReceipt.CapabilitySummary = desktopMarkUnsupported(
			result.RuntimeReceipt.CapabilitySummary, binding.publicRequest.Operation,
		)
	}
	if reason == desktopobserve.ReasonRedactionFailed {
		result.RuntimeReceipt.Redaction.Status = desktopobserve.RedactionFailed
		result.RuntimeReceipt.Quarantine.Status = desktopobserve.QuarantineBlocked
	} else if reason == desktopobserve.ReasonEvidenceQuarantined {
		result.RuntimeReceipt.Redaction.Status = desktopobserve.RedactionApplied
		result.RuntimeReceipt.Quarantine.Status = desktopobserve.QuarantineBlocked
	}
	return result, nil
}
