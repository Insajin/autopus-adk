package desktopobserve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

const DeterministicCheckSemanticLandmarks = "desktop-semantic-landmarks"

var forbiddenObservationEvidence = regexp.MustCompile(
	`(?i)(raw[_ -]?ax|raw_tree|screenshot|\.png|0x[0-9a-f]+|/users/|/tmp/|/private/var/|/var/folders/|/volumes/|/applications/|sk-proj-)`,
)

type observationEvidenceWire struct {
	SemanticProjection  json.RawMessage `json:"semantic_projection,omitempty"`
	DeterministicChecks json.RawMessage `json:"deterministic_checks"`
	RuntimeReceipt      json.RawMessage `json:"runtime_receipt"`
}

func DecodeObservationEvidence(raw []byte) (ObservationEvidence, error) {
	var wire observationEvidenceWire
	if err := decodeStrict(raw, MaxEnvelopeBytes, &wire); err != nil {
		return ObservationEvidence{}, err
	}
	keys, err := objectKeys(raw)
	if err != nil {
		return ObservationEvidence{}, err
	}
	if !hasRequiredKeys(keys, "deterministic_checks", "runtime_receipt") {
		return ObservationEvidence{}, fmt.Errorf("%w: observation evidence", ErrMissingField)
	}

	receipt, err := DecodeRuntimeReceipt(wire.RuntimeReceipt)
	if err != nil {
		return ObservationEvidence{}, err
	}
	checks, err := decodeDeterministicChecks(wire.DeterministicChecks)
	if err != nil {
		return ObservationEvidence{}, err
	}
	projection, err := decodeEvidenceProjection(wire.SemanticProjection)
	if err != nil {
		return ObservationEvidence{}, err
	}
	evidence := ObservationEvidence{
		SemanticProjection: projection, DeterministicChecks: checks, RuntimeReceipt: receipt,
	}
	if err := validateObservationEvidence(evidence); err != nil {
		return ObservationEvidence{}, err
	}
	normalizedBody, err := json.Marshal(evidence)
	if err != nil {
		return ObservationEvidence{}, fmt.Errorf("%w: observation evidence", ErrMalformedEnvelope)
	}
	if forbiddenObservationEvidence.Match(normalizedBody) {
		return ObservationEvidence{}, ErrRedactionFailed
	}
	return evidence, nil
}

func decodeEvidenceProjection(raw json.RawMessage) (*SemanticProjection, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: semantic_projection", ErrMalformedEnvelope)
	}
	var projection SemanticProjection
	if err := decodeStrict(raw, MaxEnvelopeBytes, &projection); err != nil {
		return nil, err
	}
	if err := validateProjectionJSONShape(raw); err != nil {
		return nil, err
	}
	if err := validateProjection(projection); err != nil {
		return nil, fmt.Errorf("%w: semantic_projection", ErrMalformedEnvelope)
	}
	normalized, err := NormalizeProjection(projection, func(value string) (string, error) {
		return value, nil
	})
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func decodeDeterministicChecks(raw json.RawMessage) ([]DeterministicCheck, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: deterministic_checks", ErrMissingField)
	}
	var checks []DeterministicCheck
	if err := decodeStrict(raw, MaxEnvelopeBytes, &checks); err != nil {
		return nil, err
	}
	if len(checks) != 1 || checks[0].ID != DeterministicCheckSemanticLandmarks {
		return nil, fmt.Errorf("%w: deterministic_checks", ErrMalformedEnvelope)
	}
	if checks[0].Status != CheckPassed && checks[0].Status != CheckBlocked {
		return nil, fmt.Errorf("%w: deterministic check status", ErrInvalidStatus)
	}
	return checks, nil
}

func validateObservationEvidence(evidence ObservationEvidence) error {
	check := evidence.DeterministicChecks[0]
	receipt := evidence.RuntimeReceipt
	if receipt.ReasonCode == nil {
		if evidence.SemanticProjection == nil || check.Status != CheckPassed || check.ReasonCode != nil {
			return fmt.Errorf("%w: contradictory success evidence", ErrMalformedEnvelope)
		}
		request := Request{
			ProtocolVersion: ProtocolVersion, RequestID: "evidence-validation",
			Operation: OperationGetState, Scope: ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"},
		}
		result := Result{RuntimeReceipt: receipt, Status: ResultPassed}
		if err := validateSuccessBinding(request, result, *evidence.SemanticProjection); err != nil {
			return err
		}
		return nil
	}
	if evidence.SemanticProjection != nil || check.Status != CheckBlocked {
		return fmt.Errorf("%w: contradictory failure evidence", ErrMalformedEnvelope)
	}
	if check.ReasonCode == nil || *check.ReasonCode != *receipt.ReasonCode {
		return fmt.Errorf("%w: deterministic check reason", ErrMalformedEnvelope)
	}
	return nil
}
