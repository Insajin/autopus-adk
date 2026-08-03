package promptlayer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const ompContextEvidenceFutureSkewV1 = 5 * time.Minute

func LoadOMPContextEvidenceStoreV1(
	root string,
	expectedBinding OMPContextEvidenceStoreBindingV1,
	subject OMPContextPromotionSubjectV1,
	expectedPolicy OMPContextPromotionPolicyV1,
	now time.Time,
) (OMPContextVerifiedEvidenceStoreV1, error) {
	document, current, expiresAt, err := loadOMPContextEvidenceDocumentV1(root, expectedPolicy, now)
	if err != nil {
		return OMPContextVerifiedEvidenceStoreV1{}, err
	}
	expectedBinding.PolicyDigest = hashOMPContextPromotionValueV1(expectedPolicy)
	if !equalOMPContextEvidenceBindingV1(document.Binding, expectedBinding) {
		return OMPContextVerifiedEvidenceStoreV1{}, fmt.Errorf("OMP context evidence binding mismatch")
	}
	return verifyOMPContextEvidenceDocumentV1(document, subject, expectedPolicy, current, expiresAt)
}

func LoadOMPContextEvidenceForExpectationV1(
	root string,
	expectation OMPContextEvidenceExpectationV1,
	subject OMPContextPromotionSubjectV1,
	expectedPolicy OMPContextPromotionPolicyV1,
	now time.Time,
) (OMPContextVerifiedEvidenceStoreV1, error) { // @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: public persisted-evidence admission boundary; @AX:REASON [AUTO]: expected authority and verified canary history must bind before promotion.
	document, current, expiresAt, err := loadOMPContextEvidenceDocumentV1(root, expectedPolicy, now)
	if err != nil {
		return OMPContextVerifiedEvidenceStoreV1{}, err
	}
	if !matchesOMPContextEvidenceExpectationV1(document.Binding, expectation) {
		return OMPContextVerifiedEvidenceStoreV1{}, fmt.Errorf("OMP context evidence binding mismatch")
	}
	return verifyOMPContextEvidenceDocumentV1(document, subject, expectedPolicy, current, expiresAt)
}

func loadOMPContextEvidenceDocumentV1(
	root string,
	expectedPolicy OMPContextPromotionPolicyV1,
	now time.Time,
) (OMPContextEvidenceStoreV1, time.Time, time.Time, error) { // @AX:WARN [AUTO]: Evidence loading has eight freshness and binding branches.
	// @AX:REASON [AUTO]: Stored, canonical, expected-policy, and temporal authority must agree before verification can proceed.
	if now.IsZero() {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, fmt.Errorf("OMP context evidence verification time is required")
	}
	path, err := prepareOMPContextEvidenceStorePathV1(root, false)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, err
	}
	stored, err := readOMPContextEvidenceStoreV1(path)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, err
	}
	document, err := canonicalOMPContextEvidenceStoreV1(stored)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, err
	}
	if stored.SchemaVersion != document.SchemaVersion {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, fmt.Errorf("OMP context evidence schema mismatch")
	}
	if err := validateOMPContextEvidencePolicyV1(expectedPolicy); err != nil {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, err
	}
	storedDigest := hashOMPContextPromotionValueV1(document.Policy)
	expectedDigest := hashOMPContextPromotionValueV1(expectedPolicy)
	if document.Binding.PolicyDigest != storedDigest || storedDigest != expectedDigest {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, fmt.Errorf("OMP context evidence policy mismatch")
	}
	current := now.UTC()
	expiresAt := document.Binding.CheckedAt.UTC().Add(document.Binding.ValidFor)
	if current.Before(document.Binding.CheckedAt.UTC().Add(-ompContextEvidenceFutureSkewV1)) || !current.Before(expiresAt) {
		return OMPContextEvidenceStoreV1{}, time.Time{}, time.Time{}, fmt.Errorf("OMP context evidence is stale or future-dated")
	}
	return document, current, expiresAt, nil
}

func verifyOMPContextEvidenceDocumentV1(
	document OMPContextEvidenceStoreV1,
	subject OMPContextPromotionSubjectV1,
	expectedPolicy OMPContextPromotionPolicyV1,
	current, expiresAt time.Time,
) (OMPContextVerifiedEvidenceStoreV1, error) {
	if subject.WorkspaceID != document.Binding.WorkspaceID || subject.SpecID != document.Binding.SpecID {
		return OMPContextVerifiedEvidenceStoreV1{}, fmt.Errorf("OMP context evidence subject mismatch")
	}
	validFor := expiresAt.Sub(current)
	if validFor > maxOMPContextPromotionAttestationTTL {
		validFor = maxOMPContextPromotionAttestationTTL
	}
	attestation, err := BuildOMPContextPromotionAttestationV1(OMPContextPromotionAttestationInputV1{
		Subject: subject, Policy: expectedPolicy, Rows: document.CanaryRows,
		CheckedAt: current, ValidFor: validFor,
	})
	if err != nil {
		return OMPContextVerifiedEvidenceStoreV1{}, fmt.Errorf("verify OMP context promotion evidence: %w", err)
	}
	return OMPContextVerifiedEvidenceStoreV1{
		Promotion: OMPContextPromotionEvidenceV1{
			Rows: append([]OMPContextCanaryRowV1(nil), document.CanaryRows...), Attestation: attestation,
		},
		HistoryRefs: append([]OMPContextHistoryReference(nil), document.HistoryRefs...),
	}, nil
}

func equalOMPContextEvidenceBindingV1(left, right OMPContextEvidenceStoreBindingV1) bool {
	if err := validateOMPContextEvidenceBindingV1(right); err != nil {
		return false
	}
	return left.WorkspaceID == right.WorkspaceID && left.SpecID == right.SpecID &&
		left.SnapshotHash == right.SnapshotHash && left.GitCommitHash == right.GitCommitHash &&
		left.PolicyDigest == right.PolicyDigest && left.RuntimeVersion == right.RuntimeVersion &&
		left.CheckedAt.Equal(right.CheckedAt) && left.ValidFor == right.ValidFor
}

func prepareOMPContextEvidenceStorePathV1(root string, create bool) (string, error) { // @AX:WARN [AUTO]: Rooted evidence-path admission has cyclomatic complexity 22.
	// @AX:REASON [AUTO]: Permission, symlink, containment, and directory-creation checks defend the persisted authority boundary.
	if root == "" || filepath.Clean(root) == "." {
		return "", fmt.Errorf("OMP context evidence root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve OMP context evidence root: %w", err)
	}
	rootInfo, err := os.Lstat(absolute)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("OMP context evidence root must be a regular directory")
	}
	if rootInfo.Mode().Perm()&0o022 != 0 {
		return "", fmt.Errorf("OMP context evidence root permissions are unsafe")
	}
	resolvedRoot, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve OMP context evidence root identity: %w", err)
	}
	parts := []string{".autopus", "runtime", "omp-context"}
	current := absolute
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) && create {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil {
				return "", fmt.Errorf("create OMP context evidence directory: %w", mkdirErr)
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("OMP context evidence directory is unsafe")
		}
		if index == 0 && info.Mode().Perm()&0o022 != 0 {
			return "", fmt.Errorf("OMP context evidence parent permissions are unsafe")
		}
		if index > 0 && info.Mode().Perm() != 0o700 {
			return "", fmt.Errorf("OMP context evidence directory permissions are unsafe")
		}
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr != nil || !pathContainedByOMPContextEvidenceRootV1(resolvedRoot, resolved) {
			return "", fmt.Errorf("OMP context evidence directory escapes root")
		}
	}
	return filepath.Join(current, "evidence-v1.json"), nil
}

func pathContainedByOMPContextEvidenceRootV1(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && !filepath.IsAbs(relative) && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func verifyOMPContextEvidenceFileV1(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("OMP context evidence file is unsafe")
	}
	if info.Size() <= 0 || info.Size() > ompContextEvidenceStoreMaxV1 {
		return fmt.Errorf("OMP context evidence file size is invalid")
	}
	return nil
}

func readOMPContextEvidenceStoreV1(path string) (OMPContextEvidenceStoreV1, error) {
	if err := verifyOMPContextEvidenceFileV1(path); err != nil {
		return OMPContextEvidenceStoreV1{}, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("inspect OMP context evidence: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("open OMP context evidence: %w", err)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || after.Mode().Perm() != 0o600 || !os.SameFile(before, after) {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("OMP context evidence identity changed")
	}
	body, err := io.ReadAll(io.LimitReader(file, ompContextEvidenceStoreMaxV1+1))
	if err != nil || len(body) == 0 || len(body) > ompContextEvidenceStoreMaxV1 {
		return OMPContextEvidenceStoreV1{}, fmt.Errorf("read OMP context evidence")
	}
	var document OMPContextEvidenceStoreV1
	if err := decodeStrictOMPContextEvidenceV1(body, &document); err != nil {
		return OMPContextEvidenceStoreV1{}, err
	}
	return document, nil
}

func marshalOMPContextEvidenceStoreV1(document OMPContextEvidenceStoreV1) ([]byte, error) {
	body, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode OMP context evidence: %w", err)
	}
	return append(body, '\n'), nil
}

func decodeStrictOMPContextEvidenceV1(body []byte, target any) error {
	if len(body) == 0 || !utf8.Valid(body) {
		return fmt.Errorf("decode OMP context evidence: invalid UTF-8")
	}
	if err := rejectDuplicateOMPContextEvidenceKeysV1(body); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode OMP context evidence: %w", err)
	}
	if err := requireOMPContextEvidenceEOFV1(decoder); err != nil {
		return err
	}
	return nil
}

func rejectDuplicateOMPContextEvidenceKeysV1(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := consumeUniqueOMPContextEvidenceValueV1(decoder); err != nil {
		return err
	}
	return requireOMPContextEvidenceEOFV1(decoder)
}

func consumeUniqueOMPContextEvidenceValueV1(decoder *json.Decoder) error { // @AX:WARN [AUTO]: Recursive JSON admission has eight structural branches; @AX:REASON [AUTO]: duplicate or malformed nested authority fields must fail before typed decoding.
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode OMP context evidence: %w", err)
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return fmt.Errorf("decode OMP context evidence structure")
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			keyToken, keyErr := decoder.Token()
			key, ok := keyToken.(string)
			if keyErr != nil || !ok || seen[key] {
				return fmt.Errorf("OMP context evidence contains invalid or duplicate key")
			}
			seen[key] = true
		}
		if err := consumeUniqueOMPContextEvidenceValueV1(decoder); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	expected := json.Delim(']')
	if delimiter == '{' {
		expected = '}'
	}
	if err != nil || closing != expected {
		return fmt.Errorf("decode OMP context evidence structure")
	}
	return nil
}

func requireOMPContextEvidenceEOFV1(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("OMP context evidence contains trailing JSON")
	}
	return nil
}
