package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func desktopV2MessageBody(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > desktopobserve.MaxEnvelopeBytes {
		if len(raw) > desktopobserve.MaxEnvelopeBytes {
			return nil, desktopobserve.ErrEnvelopeTooLarge
		}
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	if !utf8.Valid(raw) || raw[len(raw)-1] != '\n' {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	body := raw[:len(raw)-1]
	if len(body) == 0 || bytes.ContainsAny(body, "\r\n") {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	return body, nil
}

func decodeDesktopV2Object(raw []byte, target any, keys ...string) error {
	if len(raw) == 0 || len(raw) > desktopobserve.MaxEnvelopeBytes || !utf8.Valid(raw) {
		if len(raw) > desktopobserve.MaxEnvelopeBytes {
			return desktopobserve.ErrEnvelopeTooLarge
		}
		return desktopobserve.ErrMalformedEnvelope
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := rejectDesktopV2DuplicateKeys(decoder); err != nil {
		return err
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return fmt.Errorf("%w: private result", desktopobserve.ErrUnknownField)
		}
		return desktopobserve.ErrMalformedEnvelope
	}
	if err := desktopV2JSONEOF(decoder); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil || len(object) != len(keys) {
		return desktopobserve.ErrMissingField
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			return desktopobserve.ErrMissingField
		}
	}
	return nil
}

func rejectDesktopV2DuplicateKeys(decoder *json.Decoder) error {
	if err := consumeDesktopV2Value(decoder); err != nil {
		return err
	}
	return desktopV2JSONEOF(decoder)
}

func consumeDesktopV2Value(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return desktopobserve.ErrMalformedEnvelope
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			key, ok := keyToken.(string)
			if keyErr != nil || !ok {
				return desktopobserve.ErrMalformedEnvelope
			}
			if seen[key] {
				return desktopobserve.ErrDuplicateKey
			}
			seen[key] = true
			if err := consumeDesktopV2Value(decoder); err != nil {
				return err
			}
		}
		return consumeDesktopV2Closing(decoder, '}')
	case '[':
		for decoder.More() {
			if err := consumeDesktopV2Value(decoder); err != nil {
				return err
			}
		}
		return consumeDesktopV2Closing(decoder, ']')
	default:
		return desktopobserve.ErrMalformedEnvelope
	}
}

func consumeDesktopV2Closing(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != expected {
		return desktopobserve.ErrMalformedEnvelope
	}
	return nil
}

func desktopV2JSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		return desktopobserve.ErrMalformedEnvelope
	}
	return nil
}
