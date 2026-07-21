package orchestra

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

func DecodeReviewPrepareContractStrict(payload []byte, maxBytes int) (ReviewPrepareContract, error) {
	limit := maxBytes
	if limit <= 0 || limit > ReviewPrepareMaximumBytes {
		limit = ReviewPrepareMaximumBytes
	}
	if len(payload) == 0 || len(payload) > limit || !utf8.Valid(payload) || rejectReviewDuplicateKeys(payload) != nil {
		return ReviewPrepareContract{}, ErrReviewPrepareInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var contract ReviewPrepareContract
	if err := decoder.Decode(&contract); err != nil {
		return ReviewPrepareContract{}, ErrReviewPrepareInvalid
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ReviewPrepareContract{}, ErrReviewPrepareInvalid
	}
	if validateReviewPrepareContract(contract) != nil {
		return ReviewPrepareContract{}, ErrReviewPrepareInvalid
	}
	return contract, nil
}

func rejectReviewDuplicateKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := consumeReviewJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrReviewPrepareInvalid
	}
	return nil
}

func consumeReviewJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return keyErr
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrReviewPrepareInvalid
			}
			if _, duplicate := seen[key]; duplicate {
				return ErrReviewPrepareInvalid
			}
			seen[key] = struct{}{}
			if err := consumeReviewJSONValue(decoder); err != nil {
				return err
			}
		}
		return consumeReviewClosingDelimiter(decoder, '}')
	case '[':
		for decoder.More() {
			if err := consumeReviewJSONValue(decoder); err != nil {
				return err
			}
		}
		return consumeReviewClosingDelimiter(decoder, ']')
	default:
		return ErrReviewPrepareInvalid
	}
}

func consumeReviewClosingDelimiter(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != expected {
		return ErrReviewPrepareInvalid
	}
	return nil
}
