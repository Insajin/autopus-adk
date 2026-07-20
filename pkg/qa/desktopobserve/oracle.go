package desktopobserve

import "fmt"

type FailureCondition string

const (
	FailureProviderStart         FailureCondition = "provider_start"
	FailureOperationMissing      FailureCondition = "operation_missing"
	FailureAccessibilityDenied   FailureCondition = "accessibility_denied"
	FailureAppAliasUnmatched     FailureCondition = "app_alias_unmatched"
	FailureWindowAliasUnmatched  FailureCondition = "window_alias_unmatched"
	FailureStateRefRejected      FailureCondition = "state_ref_rejected"
	FailureLandmarksInsufficient FailureCondition = "landmarks_insufficient"
	FailureRedaction             FailureCondition = "redaction"
	FailureRawOnlyQuarantine     FailureCondition = "raw_only_quarantine"
	FailureProtocolVersion       FailureCondition = "protocol_version"
)

var failureReasons = map[FailureCondition]ReasonCode{
	FailureProviderStart:         ReasonProviderUnavailable,
	FailureOperationMissing:      ReasonCapabilityUnsupported,
	FailureAccessibilityDenied:   ReasonAccessibilityPermissionMissing,
	FailureAppAliasUnmatched:     ReasonTargetAppNotFound,
	FailureWindowAliasUnmatched:  ReasonTargetWindowNotFound,
	FailureStateRefRejected:      ReasonStaleState,
	FailureLandmarksInsufficient: ReasonSemanticProjectionUnavailable,
	FailureRedaction:             ReasonRedactionFailed,
	FailureRawOnlyQuarantine:     ReasonEvidenceQuarantined,
	FailureProtocolVersion:       ReasonProviderProtocolMismatch,
}

type Verdict string

const (
	VerdictPassed  Verdict = "passed"
	VerdictBlocked Verdict = "blocked"
)

type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckBlocked CheckStatus = "blocked"
)

type DeterministicCheck struct {
	ID         string      `json:"id"`
	Status     CheckStatus `json:"status"`
	ReasonCode *ReasonCode `json:"reason_code,omitempty"`
}

type LandmarkRequirement struct {
	Role          Role
	Name          string
	RequiredState SemanticStateKey
}

type OraclePolicy struct {
	MinimumLandmarks []LandmarkRequirement
	AllowedNames     []string
}

type OracleInput struct {
	Projection SemanticProjection
	Ledger     *StateLedger
	Policy     OraclePolicy
	Receipt    RuntimeReceipt
}

type OracleOutcome struct {
	Verdict             Verdict              `json:"verdict"`
	ReasonCode          *ReasonCode          `json:"reason_code,omitempty"`
	SemanticProjection  *SemanticProjection  `json:"semantic_projection,omitempty"`
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks"`
	RuntimeReceipt      RuntimeReceipt       `json:"runtime_receipt"`
	FailureCondition    FailureCondition     `json:"-"`
}

type FailureSignal struct {
	Condition          FailureCondition
	Provider           ProviderIdentity
	Scope              ReceiptScope
	CapabilitySummary  []CapabilityStatus
	RequestedOperation Operation
}

func EvaluateOracle(input OracleInput) (OracleOutcome, error) {
	if err := input.Receipt.Validate(); err != nil {
		return OracleOutcome{}, fmt.Errorf("oracle receipt: %w", err)
	}
	if err := validateStateReceipt(input.Receipt); err != nil {
		return OracleOutcome{}, err
	}
	if err := validateProjection(input.Projection); err != nil {
		return OracleOutcome{}, fmt.Errorf("oracle projection: %w", err)
	}
	binding := StateBinding{
		StateRef: input.Projection.StateRef, ProviderRef: input.Projection.ProviderRef,
		AppRef: input.Projection.AppRef, WindowRef: input.Projection.WindowRef,
		Digest: input.Projection.Digest,
	}
	if input.Ledger == nil || input.Ledger.Consume(binding) != nil {
		return normalizedOracleFailure(FailureStateRefRejected, input.Receipt)
	}
	if !namesAreAllowlisted(input.Projection.Root, input.Policy.AllowedNames) {
		return normalizedOracleFailure(FailureRedaction, input.Receipt)
	}
	if !meetsLandmarkPolicy(input.Projection.Root, input.Policy) {
		return normalizedOracleFailure(FailureLandmarksInsufficient, input.Receipt)
	}
	projection := cloneProjection(input.Projection)
	return OracleOutcome{
		Verdict:            VerdictPassed,
		SemanticProjection: &projection,
		DeterministicChecks: []DeterministicCheck{{
			ID: "desktop-semantic-landmarks", Status: CheckPassed,
		}},
		RuntimeReceipt: cloneRuntimeReceipt(input.Receipt),
	}, nil
}

func NormalizeFailure(signal FailureSignal) (OracleOutcome, error) {
	reason, ok := failureReasons[signal.Condition]
	if !ok || !validProvider(signal.Provider) || !validReceiptScope(signal.Scope) ||
		!validCapabilities(signal.CapabilitySummary) {
		return OracleOutcome{}, ErrFailureSignalInvalid
	}
	if signal.Condition == FailureOperationMissing &&
		(!IsReadOnlyOperation(signal.RequestedOperation) ||
			capabilityState(signal.CapabilitySummary, signal.RequestedOperation) != CapabilityUnsupported) {
		return OracleOutcome{}, ErrFailureSignalInvalid
	}
	receipt := failureReceipt(signal.Provider, signal.Scope, signal.CapabilitySummary, reason)
	if err := receipt.Validate(); err != nil {
		return OracleOutcome{}, fmt.Errorf("failure receipt: %w", err)
	}
	return OracleOutcome{
		Verdict: VerdictBlocked, ReasonCode: &reason,
		DeterministicChecks: []DeterministicCheck{{
			ID: "desktop-semantic-landmarks", Status: CheckBlocked, ReasonCode: &reason,
		}},
		RuntimeReceipt: receipt, FailureCondition: signal.Condition,
	}, nil
}

func normalizedOracleFailure(condition FailureCondition, receipt RuntimeReceipt) (OracleOutcome, error) {
	return NormalizeFailure(FailureSignal{
		Condition: condition, Provider: receipt.Provider, Scope: receipt.Scope,
		CapabilitySummary: receipt.CapabilitySummary,
	})
}

func failureReceipt(
	provider ProviderIdentity,
	scope ReceiptScope,
	capabilities []CapabilityStatus,
	reason ReasonCode,
) RuntimeReceipt {
	nextStep := NextStep(reason)
	receipt := RuntimeReceipt{
		SchemaVersion: RuntimeReceiptSchemaVersion, Provider: provider, Scope: scope,
		CapabilitySummary: cloneCapabilities(capabilities), ReasonCode: &reason, NextStep: &nextStep,
		Redaction:  RedactionReceipt{Status: RedactionNotRequired},
		Quarantine: QuarantineReceipt{Status: QuarantineEmpty},
	}
	switch reason {
	case ReasonRedactionFailed:
		receipt.Redaction.Status = RedactionFailed
		receipt.Quarantine.Status = QuarantineBlocked
	case ReasonEvidenceQuarantined:
		receipt.Redaction.Status = RedactionApplied
		receipt.Quarantine.Status = QuarantineLocalOnly
	}
	return receipt
}

func supportedCapabilities() []CapabilityStatus {
	operations := ReadOnlyOperations()
	capabilities := make([]CapabilityStatus, 0, len(operations))
	for _, operation := range operations {
		capabilities = append(capabilities, CapabilityStatus{
			Name: operation, Status: CapabilitySupported,
		})
	}
	return capabilities
}

func unsupportedCapabilities() []CapabilityStatus {
	capabilities := supportedCapabilities()
	for index := range capabilities {
		capabilities[index].Status = CapabilityUnsupported
	}
	return capabilities
}

func capabilityState(capabilities []CapabilityStatus, operation Operation) CapabilityState {
	for _, capability := range capabilities {
		if capability.Name == operation {
			return capability.Status
		}
	}
	return ""
}

func validateStateReceipt(receipt RuntimeReceipt) error {
	if receipt.ReasonCode != nil || receipt.Redaction.Status != RedactionApplied ||
		receipt.Quarantine.Status != QuarantineCleared {
		return ErrRedactionFailed
	}
	return nil
}

func namesAreAllowlisted(root SemanticNode, allowedNames []string) bool {
	if len(allowedNames) == 0 {
		return false
	}
	allowed := make(map[string]struct{}, len(allowedNames))
	for _, name := range allowedNames {
		normalized, err := normalizeText(name)
		if err != nil {
			return false
		}
		allowed[normalized] = struct{}{}
	}
	var walk func(SemanticNode) bool
	walk = func(node SemanticNode) bool {
		if _, ok := allowed[node.Name]; !ok {
			return false
		}
		for _, child := range node.Children {
			if !walk(child) {
				return false
			}
		}
		return true
	}
	return walk(root)
}

func meetsLandmarkPolicy(root SemanticNode, policy OraclePolicy) bool {
	if len(policy.MinimumLandmarks) == 0 {
		return false
	}
	for _, requirement := range policy.MinimumLandmarks {
		if !containsLandmark(root, requirement) {
			return false
		}
	}
	return true
}

func containsLandmark(node SemanticNode, requirement LandmarkRequirement) bool {
	if node.Role == requirement.Role && node.Name == requirement.Name &&
		stateSatisfies(node.SemanticState, requirement.RequiredState) {
		return true
	}
	for _, child := range node.Children {
		if containsLandmark(child, requirement) {
			return true
		}
	}
	return false
}

func stateSatisfies(state SemanticState, key SemanticStateKey) bool {
	return map[SemanticStateKey]bool{
		StateEnabled: boolState(state.Enabled), StateFocused: boolState(state.Focused),
		StateSelected: boolState(state.Selected), StateExpanded: boolState(state.Expanded),
	}[key]
}

func boolState(value *bool) bool {
	return value != nil && *value
}
