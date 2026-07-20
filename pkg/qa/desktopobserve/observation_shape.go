package desktopobserve

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func validateProjectionJSONShape(raw json.RawMessage) error {
	keys, err := objectKeys(raw)
	if err != nil {
		return err
	}
	if !hasExactKeys(keys,
		"schema_version", "provider_ref", "app_ref", "window_ref", "state_ref", "digest", "root",
	) {
		return fmt.Errorf("%w: semantic_projection", ErrMissingField)
	}
	for _, key := range []string{
		"schema_version", "provider_ref", "app_ref", "window_ref", "state_ref", "digest", "root",
	} {
		if isJSONNull(keys[key]) {
			return fmt.Errorf("%w: semantic_projection.%s", ErrMalformedEnvelope, key)
		}
	}
	return validateSemanticNodeJSONShape(keys["root"])
}

func validateSemanticNodeJSONShape(raw json.RawMessage) error {
	keys, err := objectKeys(raw)
	if err != nil {
		return err
	}
	if !hasRequiredKeys(keys, "node_ref", "role", "name", "semantic_state") {
		return fmt.Errorf("%w: semantic node", ErrMissingField)
	}
	for _, key := range []string{"node_ref", "role", "name", "semantic_state"} {
		if isJSONNull(keys[key]) {
			return fmt.Errorf("%w: semantic node.%s", ErrMalformedEnvelope, key)
		}
	}
	if err := validateSemanticStateJSONShape(keys["semantic_state"]); err != nil {
		return err
	}
	if frame, present := keys["frame"]; present {
		if err := validateFrameJSONShape(frame); err != nil {
			return err
		}
	}
	if actions, present := keys["advertised_actions"]; present && isJSONNull(actions) {
		return fmt.Errorf("%w: advertised_actions", ErrMalformedEnvelope)
	}
	if children, present := keys["children"]; present {
		if isJSONNull(children) {
			return fmt.Errorf("%w: children", ErrMalformedEnvelope)
		}
		var values []json.RawMessage
		if err := decodeStrict(children, MaxEnvelopeBytes, &values); err != nil {
			return err
		}
		for _, child := range values {
			if err := validateSemanticNodeJSONShape(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSemanticStateJSONShape(raw json.RawMessage) error {
	keys, err := objectKeys(raw)
	if err != nil {
		return err
	}
	for key, value := range keys {
		switch key {
		case "enabled", "expanded", "focused", "selected":
			if isJSONNull(value) {
				return fmt.Errorf("%w: semantic_state.%s", ErrMalformedEnvelope, key)
			}
		default:
			return fmt.Errorf("%w: semantic_state.%s", ErrUnknownField, key)
		}
	}
	return nil
}

func validateFrameJSONShape(raw json.RawMessage) error {
	keys, err := objectKeys(raw)
	if err != nil {
		return err
	}
	if !hasExactKeys(keys, "x", "y", "width", "height") {
		return fmt.Errorf("%w: frame", ErrMissingField)
	}
	for key, value := range keys {
		if isJSONNull(value) {
			return fmt.Errorf("%w: frame.%s", ErrMalformedEnvelope, key)
		}
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
