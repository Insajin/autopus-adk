package omp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"
)

const (
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: model receipts are capped at 1 MiB and remain valid for 24 hours.
	maxOMPModelReceiptBytes = 1 << 20
	ompModelReceiptValidFor = 24 * time.Hour
)

// LoadOMPModelResolutionReceipt loads the current, canonical model-routing receipt.
func LoadOMPModelResolutionReceipt(root string) (OMPModelResolutionReceipt, error) {
	return LoadOMPModelResolutionReceiptAt(root, time.Now().UTC(), ompModelReceiptValidFor)
}

// LoadOMPModelResolutionReceiptAt loads a receipt at an explicit freshness boundary.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: rooted receipt loading is the persisted phase-model authority boundary.
// @AX:REASON [AUTO]: File identity, mode, strict JSON, canonical digest, and freshness checks must remain fail closed.
// @AX:WARN [AUTO]: Receipt loading has cyclomatic complexity 17 across rooted I/O, canonicalization, and freshness checks.
// @AX:REASON [AUTO]: Any malformed, drifted, stale, or non-canonical receipt must return no usable model routes.
func LoadOMPModelResolutionReceiptAt(
	root string,
	now time.Time,
	validFor time.Duration,
) (receipt OMPModelResolutionReceipt, returnErr error) {
	if now.IsZero() || validFor <= 0 {
		return OMPModelResolutionReceipt{}, fmt.Errorf("receipt freshness boundary is invalid")
	}
	workspace, err := openOMPRootedWorkspace(root)
	if err != nil {
		return OMPModelResolutionReceipt{}, err
	}
	defer func() {
		joinOMPRootedCloseError(&returnErr, workspace.Close())
		if returnErr != nil {
			receipt = OMPModelResolutionReceipt{}
		}
	}()

	data, _, err := workspace.readOwnerOnlyFile(OMPModelReceiptRelativePath, maxOMPModelReceiptBytes)
	if err != nil {
		return OMPModelResolutionReceipt{}, fmt.Errorf("read OMP model receipt: %w", err)
	}
	if err := rejectDuplicateOMPModelReceiptJSON(data); err != nil {
		return OMPModelResolutionReceipt{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return OMPModelResolutionReceipt{}, fmt.Errorf("decode OMP model receipt: %w", err)
	}
	if err := requireOMPModelDoctorJSONEOF(decoder); err != nil {
		return OMPModelResolutionReceipt{}, fmt.Errorf("decode OMP model receipt: %w", err)
	}
	if err := validateOMPModelReceipt(receipt); err != nil {
		return OMPModelResolutionReceipt{}, fmt.Errorf("validate OMP model receipt: %w", err)
	}

	canonical, _, err := CanonicalOMPModelResolutionReceipt(receipt)
	if err != nil {
		return OMPModelResolutionReceipt{}, fmt.Errorf("canonicalize OMP model receipt: %w", err)
	}
	if canonical.ResolutionDigest != receipt.ResolutionDigest || !reflect.DeepEqual(canonical, receipt) {
		return OMPModelResolutionReceipt{}, fmt.Errorf("OMP model receipt is not canonical")
	}
	if err := validateOMPModelReceiptWorkspaceBinding(workspace, receipt); err != nil {
		return OMPModelResolutionReceipt{}, err
	}
	now = now.UTC()
	if receipt.GeneratedAt.After(now) || now.Sub(receipt.GeneratedAt) > validFor {
		return OMPModelResolutionReceipt{}, fmt.Errorf("OMP model receipt is stale")
	}
	return cloneOMPModelResolutionReceipt(receipt), nil
}

func rejectDuplicateOMPModelReceiptJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanOMPModelJSONValue(decoder); err != nil {
		return fmt.Errorf("decode OMP model receipt: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode OMP model receipt: unexpected trailing JSON value")
		}
		return fmt.Errorf("decode OMP model receipt: %w", err)
	}
	return nil
}

// @AX:WARN [AUTO]: Recursive duplicate-key scanning has cyclomatic complexity 16.
// @AX:REASON [AUTO]: Nested objects and arrays must reject duplicate routing fields before typed receipt decoding.
func scanOMPModelJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanOMPModelJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("invalid JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanOMPModelJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("invalid JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func cloneOMPModelResolutionReceipt(receipt OMPModelResolutionReceipt) OMPModelResolutionReceipt {
	receipt.Activation.Argv = append([]string(nil), receipt.Activation.Argv...)
	receipt.Roles = append([]OMPModelRoleReceipt(nil), receipt.Roles...)
	for index := range receipt.Roles {
		attempts := receipt.Roles[index].FallbackAttempts
		if attempts != nil {
			receipt.Roles[index].FallbackAttempts = append(
				make([]OMPModelFallbackAttemptReceipt, 0, len(attempts)),
				attempts...,
			)
		}
	}
	return receipt
}
