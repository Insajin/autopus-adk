package desktopobserve

import (
	"fmt"
	"regexp"
	"strings"
)

type RuntimeReceipt struct {
	SchemaVersion     string             `json:"schema_version"`
	Provider          ProviderIdentity   `json:"provider"`
	Scope             ReceiptScope       `json:"scope"`
	CapabilitySummary []CapabilityStatus `json:"capability_summary"`
	ReasonCode        *ReasonCode        `json:"reason_code"`
	NextStep          *string            `json:"next_step"`
	Redaction         RedactionReceipt   `json:"redaction"`
	Quarantine        QuarantineReceipt  `json:"quarantine"`
}

var safeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-z0-9][a-z0-9.-]*)?$`)
var safePublicRefPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func (receipt RuntimeReceipt) Validate() error {
	if receipt.SchemaVersion != RuntimeReceiptSchemaVersion {
		return fmt.Errorf("%w: schema_version", ErrMissingField)
	}
	if !validProvider(receipt.Provider) {
		return fmt.Errorf("%w: provider", ErrMalformedEnvelope)
	}
	if !validReceiptScope(receipt.Scope) {
		return fmt.Errorf("%w: scope", ErrMalformedEnvelope)
	}
	if !validCapabilities(receipt.CapabilitySummary) {
		return fmt.Errorf("%w: capability_summary", ErrMalformedEnvelope)
	}
	if !validRedactionStatus(receipt.Redaction.Status) ||
		!validQuarantineStatus(receipt.Quarantine.Status) {
		return fmt.Errorf("%w: receipt status", ErrMalformedEnvelope)
	}
	return validateReceiptOutcome(receipt)
}

func DecodeRuntimeReceipt(raw []byte) (RuntimeReceipt, error) {
	var receipt RuntimeReceipt
	if err := decodeStrict(raw, MaxEnvelopeBytes, &receipt); err != nil {
		return RuntimeReceipt{}, err
	}
	keys, err := objectKeys(raw)
	if err != nil {
		return RuntimeReceipt{}, err
	}
	if !hasExactKeys(keys, "schema_version", "provider", "scope", "capability_summary",
		"reason_code", "next_step", "redaction", "quarantine") {
		return RuntimeReceipt{}, fmt.Errorf("%w: runtime_receipt", ErrMissingField)
	}
	if err := receipt.Validate(); err != nil {
		return RuntimeReceipt{}, err
	}
	return receipt, nil
}

func validProvider(provider ProviderIdentity) bool {
	if provider.Name != "autopus-desktop-local" && provider.Name != "orca-computer-use-macos" {
		return false
	}
	return safeVersion.MatchString(provider.Version) && provider.ProtocolVersion == ProtocolVersion
}

func validReceiptScope(scope ReceiptScope) bool {
	switch scope.Kind {
	case ScopeProvider:
		return scope.PublicRef == "autopus-desktop-local" || scope.PublicRef == "orca-computer-use-macos"
	case ScopeApplication:
		return scope.PublicRef == "autopus-desktop"
	case ScopeWindow:
		return scope.PublicRef == "main-window"
	case ScopeState:
		return validOpaqueRef(scope.PublicRef, "state-")
	default:
		return false
	}
}

func validCapabilities(capabilities []CapabilityStatus) bool {
	operations := ReadOnlyOperations()
	if len(capabilities) != len(operations) {
		return false
	}
	for index, capability := range capabilities {
		if capability.Name != operations[index] ||
			(capability.Status != CapabilitySupported && capability.Status != CapabilityUnsupported) {
			return false
		}
	}
	return true
}

func validateReceiptOutcome(receipt RuntimeReceipt) error {
	if receipt.ReasonCode == nil {
		if receipt.NextStep != nil {
			return fmt.Errorf("%w: success next_step", ErrMalformedEnvelope)
		}
		if (receipt.Redaction.Status == RedactionNotRequired && receipt.Quarantine.Status == QuarantineEmpty) ||
			(receipt.Redaction.Status == RedactionApplied && receipt.Quarantine.Status == QuarantineCleared) {
			return nil
		}
		return fmt.Errorf("%w: inconsistent success receipt", ErrMalformedEnvelope)
	}
	reason := *receipt.ReasonCode
	if !validReason(reason) || receipt.NextStep == nil || *receipt.NextStep != NextStep(reason) {
		return fmt.Errorf("%w: normalized failure", ErrMalformedEnvelope)
	}
	switch reason {
	case ReasonRedactionFailed:
		if receipt.Redaction.Status != RedactionFailed || receipt.Quarantine.Status != QuarantineBlocked {
			return fmt.Errorf("%w: redaction failure status", ErrMalformedEnvelope)
		}
	case ReasonEvidenceQuarantined:
		if receipt.Redaction.Status != RedactionApplied ||
			(receipt.Quarantine.Status != QuarantineLocalOnly && receipt.Quarantine.Status != QuarantineBlocked) {
			return fmt.Errorf("%w: quarantine failure status", ErrMalformedEnvelope)
		}
	default:
		if receipt.Redaction.Status != RedactionNotRequired || receipt.Quarantine.Status != QuarantineEmpty {
			return fmt.Errorf("%w: ordinary failure status", ErrMalformedEnvelope)
		}
	}
	return nil
}

func validRedactionStatus(status RedactionStatus) bool {
	return status == RedactionApplied || status == RedactionNotRequired || status == RedactionFailed
}

func validQuarantineStatus(status QuarantineStatus) bool {
	return status == QuarantineEmpty || status == QuarantineLocalOnly ||
		status == QuarantineCleared || status == QuarantineBlocked
}

func validOpaqueRef(value, prefix string) bool {
	return len(value) > len(prefix) && len(value) <= 96 &&
		strings.HasPrefix(value, prefix) && safePublicRefPattern.MatchString(value)
}
