package run

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func mapDesktopV2Payload(
	operation desktopobserve.Operation,
	raw json.RawMessage,
) (json.RawMessage, error) {
	var publicPayload any
	switch operation {
	case desktopobserve.OperationCapabilities:
		var payload struct {
			Capabilities []string `json:"capabilities"`
		}
		if err := decodeDesktopV2Object(raw, &payload, "capabilities"); err != nil ||
			!slices.Equal(payload.Capabilities, desktopV2Operations) {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		publicPayload = struct {
			Capabilities []desktopobserve.CapabilityStatus `json:"capabilities"`
		}{Capabilities: desktopSupportedCapabilities()}
	case desktopobserve.OperationPermissions:
		var payload struct {
			Accessibility string  `json:"accessibility"`
			NextStep      *string `json:"next_step"`
		}
		if err := decodeDesktopV2Object(raw, &payload, "accessibility", "next_step"); err != nil ||
			payload.Accessibility != "granted" || payload.NextStep != nil {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		publicPayload = desktopobserve.PermissionResult{AccessibilityGranted: true}
	case desktopobserve.OperationListApps:
		var payload struct {
			AppRefs []string `json:"app_refs"`
		}
		if err := decodeDesktopV2Object(raw, &payload, "app_refs"); err != nil ||
			!slices.Equal(payload.AppRefs, []string{"autopus-desktop"}) {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		publicPayload = struct {
			Apps []desktopobserve.AppSummary `json:"apps"`
		}{Apps: []desktopobserve.AppSummary{{AppRef: "autopus-desktop"}}}
	case desktopobserve.OperationListWindows:
		var payload struct {
			WindowRefs []string `json:"window_refs"`
		}
		if err := decodeDesktopV2Object(raw, &payload, "window_refs"); err != nil ||
			!slices.Equal(payload.WindowRefs, []string{"main-window"}) {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		publicPayload = struct {
			Windows []desktopobserve.WindowSummary `json:"windows"`
		}{Windows: []desktopobserve.WindowSummary{{WindowRef: "main-window"}}}
	case desktopobserve.OperationGetState:
		var payload struct {
			SemanticProjection json.RawMessage `json:"semantic_projection"`
		}
		if err := decodeDesktopV2Object(raw, &payload, "semantic_projection"); err != nil ||
			len(payload.SemanticProjection) == 0 ||
			bytes.Equal(bytes.TrimSpace(payload.SemanticProjection), []byte("null")) {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
		projection, err := mapDesktopV2Projection(payload.SemanticProjection)
		if err != nil {
			return nil, err
		}
		publicPayload = struct {
			SemanticProjection desktopobserve.SemanticProjection `json:"semantic_projection"`
		}{SemanticProjection: projection}
	default:
		return nil, desktopobserve.ErrUnsupportedOperation
	}
	encoded, err := json.Marshal(publicPayload)
	if err != nil || len(encoded) > desktopobserve.MaxEnvelopeBytes {
		return nil, desktopobserve.ErrEnvelopeTooLarge
	}
	return encoded, nil
}
