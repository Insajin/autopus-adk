package desktopobserve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func decodeStrict(raw []byte, limit int, target any) error {
	if len(raw) == 0 || !utf8.Valid(raw) {
		return ErrMalformedEnvelope
	}
	if len(raw) > limit {
		return ErrEnvelopeTooLarge
	}
	if err := rejectDuplicateKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return fmt.Errorf("%w: %v", ErrUnknownField, err)
		}
		return fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func rejectDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeUniqueValue(decoder); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func consumeUniqueValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	switch delimiter {
	case '{':
		return consumeUniqueObject(decoder)
	case '[':
		for decoder.More() {
			if err := consumeUniqueValue(decoder); err != nil {
				return err
			}
		}
		return consumeClosingDelimiter(decoder, ']')
	default:
		return ErrMalformedEnvelope
	}
}

func consumeUniqueObject(decoder *json.Decoder) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
		}
		key, ok := token.(string)
		if !ok {
			return ErrMalformedEnvelope
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%w: %s", ErrDuplicateKey, key)
		}
		seen[key] = struct{}{}
		if err := consumeUniqueValue(decoder); err != nil {
			return err
		}
	}
	return consumeClosingDelimiter(decoder, '}')
}

func consumeClosingDelimiter(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != expected {
		return ErrMalformedEnvelope
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		return ErrMalformedEnvelope
	}
	return nil
}

func objectKeys(raw []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, ErrMalformedEnvelope
	}
	return object, nil
}

func hasExactKeys(keys map[string]json.RawMessage, expected ...string) bool {
	if len(keys) != len(expected) {
		return false
	}
	for _, key := range expected {
		if _, ok := keys[key]; !ok {
			return false
		}
	}
	return true
}

func hasRequiredKeys(keys map[string]json.RawMessage, required ...string) bool {
	for _, key := range required {
		if _, ok := keys[key]; !ok {
			return false
		}
	}
	return true
}
