package run

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

const (
	desktopV2Protocol    = 2
	desktopV2StatusOK    = "ok"
	desktopV2StatusError = "error"
)

var desktopV2Operations = []string{
	"capabilities", "get_state", "list_apps", "list_windows", "permissions",
}

type desktopV2Scope struct {
	Kind      string `json:"kind"`
	PublicRef string `json:"public_ref"`
}

type desktopV2Request struct {
	ProtocolVersion  int            `json:"protocol_version"`
	RequestID        string         `json:"request_id"`
	Operation        string         `json:"operation"`
	Scope            desktopV2Scope `json:"scope"`
	ExpectedStateRef string         `json:"expected_state_ref,omitempty"`
}

type desktopV2Binding struct {
	publicRequest    desktopobserve.Request
	publicRequestRaw []byte
	privateRequest   desktopV2Request
}

type desktopV2ResultWire struct {
	ProtocolVersion int             `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Status          string          `json:"status"`
	RuntimeReceipt  json.RawMessage `json:"runtime_receipt"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

func encodeDesktopV2Request(publicRaw []byte) ([]byte, desktopV2Binding, error) {
	body, err := desktopV2MessageBody(publicRaw)
	if err != nil {
		return nil, desktopV2Binding{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := rejectDesktopV2DuplicateKeys(decoder); err != nil {
		return nil, desktopV2Binding{}, err
	}
	request, err := desktopobserve.DecodeRequest(publicRaw)
	if err != nil {
		return nil, desktopV2Binding{}, err
	}
	privateScope := desktopV2Scope{Kind: "provider", PublicRef: "provider_selected"}
	switch request.Operation {
	case desktopobserve.OperationListWindows:
		privateScope = desktopV2Scope{Kind: "application", PublicRef: "autopus-desktop"}
	case desktopobserve.OperationGetState:
		privateScope = desktopV2Scope{Kind: "window", PublicRef: "main-window"}
	}
	idDigest := sha256.Sum256([]byte(request.RequestID))
	privateRequest := desktopV2Request{
		ProtocolVersion: desktopV2Protocol,
		RequestID:       "req_adk_" + hex.EncodeToString(idDigest[:]),
		Operation:       string(request.Operation),
		Scope:           privateScope,
	}
	if request.ExpectedStateRef != "" {
		suffix := request.ExpectedStateRef[len("state-"):]
		if !desktopV2DigestPattern.MatchString(suffix) {
			return nil, desktopV2Binding{}, desktopobserve.ErrMalformedEnvelope
		}
		privateRequest.ExpectedStateRef = "state_v2_" + suffix
	}
	encoded, err := json.Marshal(privateRequest)
	if err != nil {
		return nil, desktopV2Binding{}, desktopobserve.ErrMalformedEnvelope
	}
	encoded = append(encoded, '\n')
	if len(encoded) > desktopobserve.MaxEnvelopeBytes {
		return nil, desktopV2Binding{}, desktopobserve.ErrEnvelopeTooLarge
	}
	return encoded, desktopV2Binding{
		publicRequest: request, publicRequestRaw: append([]byte(nil), publicRaw...),
		privateRequest: privateRequest,
	}, nil
}

func translateDesktopV2Result(
	binding desktopV2Binding,
	privateRaw []byte,
) ([]byte, string, error) {
	body, err := desktopV2MessageBody(privateRaw)
	if err != nil {
		return nil, "", err
	}
	var wire desktopV2ResultWire
	errWithPayload := decodeDesktopV2Object(
		body, &wire, "protocol_version", "request_id", "status", "runtime_receipt", "payload",
	)
	if errWithPayload != nil {
		if err := decodeDesktopV2Object(
			body, &wire, "protocol_version", "request_id", "status", "runtime_receipt",
		); err != nil {
			return nil, "", errWithPayload
		}
	}
	if wire.ProtocolVersion != desktopV2Protocol {
		return nil, "", desktopobserve.ErrProtocolMismatch
	}
	if wire.RequestID != binding.privateRequest.RequestID {
		return nil, "", desktopobserve.ErrRequestIDMismatch
	}
	if wire.Status != desktopV2StatusOK && wire.Status != desktopV2StatusError {
		return nil, "", desktopobserve.ErrInvalidStatus
	}
	if (wire.Status == desktopV2StatusOK) != (len(wire.Payload) != 0) ||
		bytes.Equal(bytes.TrimSpace(wire.Payload), []byte("null")) {
		return nil, "", desktopobserve.ErrMalformedEnvelope
	}
	receipt, err := decodeDesktopV2Receipt(wire.RuntimeReceipt, binding, wire.Status)
	if err != nil {
		return nil, "", err
	}
	publicResult, err := mapDesktopV2Result(binding, wire, receipt)
	if err != nil {
		return nil, "", err
	}
	publicRaw, err := json.Marshal(publicResult)
	if err != nil {
		return nil, "", desktopobserve.ErrMalformedEnvelope
	}
	publicRaw = append(publicRaw, '\n')
	if len(publicRaw) > desktopobserve.MaxEnvelopeBytes {
		return nil, "", desktopobserve.ErrEnvelopeTooLarge
	}
	if _, err := desktopobserve.DecodeExchange(binding.publicRequestRaw, publicRaw); err != nil {
		return nil, "", err
	}
	return publicRaw, wire.Status, nil
}
