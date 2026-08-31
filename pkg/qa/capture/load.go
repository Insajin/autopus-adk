package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// MaxIndexBytes bounds the producer-authored index. Capture indexes are
// summaries with bounded per-step evidence; anything larger is a producer bug or
// a hostile input, not a bigger test suite.
const MaxIndexBytes = 4 << 20

// LoadIndex reads and strictly decodes a capture index.
//
// Strictness is the point of this contract: an unknown field means the producer
// and the harness disagree about what was captured, and a duplicate key means
// the document is ambiguous. Both fail closed rather than silently dropping
// evidence, which is exactly the failure mode the previous magic-artifact-kind
// convention had.
func LoadIndex(path string) (Index, error) {
	file, err := os.Open(path)
	if err != nil {
		return Index{}, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, MaxIndexBytes+1))
	if err != nil {
		return Index{}, err
	}
	if len(body) > MaxIndexBytes {
		return Index{}, fmt.Errorf("capture index exceeds %d bytes", MaxIndexBytes)
	}
	return DecodeIndex(body)
}

// DecodeIndex strictly decodes capture index bytes.
func DecodeIndex(body []byte) (Index, error) {
	if err := rejectDuplicateKeys(body); err != nil {
		return Index{}, fmt.Errorf("parse capture index: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var index Index
	if err := decoder.Decode(&index); err != nil {
		return Index{}, fmt.Errorf("parse capture index: %w", err)
	}
	if decoder.More() {
		return Index{}, fmt.Errorf("parse capture index: trailing content after document")
	}
	return index, nil
}

// rejectDuplicateKeys walks the JSON token stream and refuses any object that
// repeats a key. encoding/json silently keeps the last value, which would let a
// producer hide a failing step behind a second, passing one.
func rejectDuplicateKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	return walkValue(decoder)
}

// walkValue consumes exactly one JSON value, rejecting duplicate object keys.
func walkValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("unexpected object key token %v", keyToken)
			}
			if keys[key] {
				return fmt.Errorf("duplicate key %q", key)
			}
			keys[key] = true
			if err := walkValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := walkValue(decoder); err != nil {
				return err
			}
		}
	}
	// Consume the matching closing delimiter.
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}
