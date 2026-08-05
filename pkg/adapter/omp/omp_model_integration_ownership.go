package omp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

const (
	OMPModelProjectOwnershipRelativePath = ".autopus/omp-model-project-ownership-v1.json"
	ompModelProjectOwnershipSchema       = "autopus.omp-model-project-ownership.v1"
)

type ompModelProjectOwnedKey struct {
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
}

type ompModelProjectOwnership struct {
	SchemaVersion            string                    `json:"schema_version"`
	Salt                     string                    `json:"salt"`
	OriginalMissing          bool                      `json:"original_missing"`
	OriginalCommitment       string                    `json:"original_commitment"`
	InitialEmittedCommitment string                    `json:"initial_emitted_commitment"`
	InitialManagedKeys       []ompModelProjectOwnedKey `json:"initial_managed_keys"`
	LastEmittedCommitment    string                    `json:"last_emitted_commitment"`
	ManagedKeys              []ompModelProjectOwnedKey `json:"managed_keys"`
	LedgerDigest             string                    `json:"ledger_digest"`
}

type ompModelProjectOwnershipBody struct {
	SchemaVersion            string                    `json:"schema_version"`
	Salt                     string                    `json:"salt"`
	OriginalMissing          bool                      `json:"original_missing"`
	OriginalCommitment       string                    `json:"original_commitment"`
	InitialEmittedCommitment string                    `json:"initial_emitted_commitment"`
	InitialManagedKeys       []ompModelProjectOwnedKey `json:"initial_managed_keys"`
	LastEmittedCommitment    string                    `json:"last_emitted_commitment"`
	ManagedKeys              []ompModelProjectOwnedKey `json:"managed_keys"`
}

func newOMPModelProjectOwnership(
	original []byte,
	originalMissing bool,
	emitted []byte,
	fingerprints map[string]string,
) (ompModelProjectOwnership, []byte, error) {
	saltBytes := make([]byte, 32)
	if _, err := rand.Read(saltBytes); err != nil {
		return ompModelProjectOwnership{}, nil, fmt.Errorf("generate project ownership salt: %w", err)
	}
	keys := ompModelProjectOwnedKeys(fingerprints)
	ownership := ompModelProjectOwnership{
		SchemaVersion: ompModelProjectOwnershipSchema,
		Salt:          hex.EncodeToString(saltBytes), OriginalMissing: originalMissing,
		OriginalCommitment:       ompModelProjectCommitment(saltBytes, original, originalMissing),
		InitialEmittedCommitment: ompModelProjectCommitment(saltBytes, emitted, false),
		InitialManagedKeys:       append([]ompModelProjectOwnedKey(nil), keys...),
		LastEmittedCommitment:    ompModelProjectCommitment(saltBytes, emitted, false),
		ManagedKeys:              keys,
	}
	return canonicalOMPModelProjectOwnership(ownership)
}

func updateOMPModelProjectOwnership(
	ownership ompModelProjectOwnership,
	emitted []byte,
	fingerprints map[string]string,
) (ompModelProjectOwnership, []byte, error) {
	salt, err := decodeOMPModelProjectSalt(ownership.Salt)
	if err != nil {
		return ompModelProjectOwnership{}, nil, err
	}
	ownership.LastEmittedCommitment = ompModelProjectCommitment(salt, emitted, false)
	ownership.ManagedKeys = ompModelProjectOwnedKeys(fingerprints)
	return canonicalOMPModelProjectOwnership(ownership)
}

func canonicalOMPModelProjectOwnership(
	ownership ompModelProjectOwnership,
) (ompModelProjectOwnership, []byte, error) {
	ownership.SchemaVersion = ompModelProjectOwnershipSchema
	ownership.LedgerDigest = ""
	sort.Slice(ownership.InitialManagedKeys, func(i, j int) bool {
		return ownership.InitialManagedKeys[i].Path < ownership.InitialManagedKeys[j].Path
	})
	sort.Slice(ownership.ManagedKeys, func(i, j int) bool {
		return ownership.ManagedKeys[i].Path < ownership.ManagedKeys[j].Path
	})
	if err := validateOMPModelProjectOwnership(ownership); err != nil {
		return ompModelProjectOwnership{}, nil, err
	}
	body := ompModelProjectOwnershipBody{
		SchemaVersion: ownership.SchemaVersion, Salt: ownership.Salt,
		OriginalMissing: ownership.OriginalMissing, OriginalCommitment: ownership.OriginalCommitment,
		InitialEmittedCommitment: ownership.InitialEmittedCommitment,
		InitialManagedKeys:       ownership.InitialManagedKeys,
		LastEmittedCommitment:    ownership.LastEmittedCommitment,
		ManagedKeys:              ownership.ManagedKeys,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return ompModelProjectOwnership{}, nil, fmt.Errorf("encode project ownership body: %w", err)
	}
	ownership.LedgerDigest = OMPModelSHA256(bodyBytes)
	data, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return ompModelProjectOwnership{}, nil, fmt.Errorf("encode project ownership ledger: %w", err)
	}
	return ownership, append(data, '\n'), nil
}

func readOMPModelProjectOwnership(root string) (ownership ompModelProjectOwnership, exists bool, returnErr error) {
	workspace, err := openOMPRootedWorkspace(root)
	if err != nil {
		return ompModelProjectOwnership{}, false, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, workspace.Close()) }()
	return readOMPModelProjectOwnershipAt(workspace)
}

func ompModelProjectOriginLedgerBytes(ownership ompModelProjectOwnership) ([]byte, error) {
	ownership.LastEmittedCommitment = ownership.InitialEmittedCommitment
	ownership.ManagedKeys = append([]ompModelProjectOwnedKey(nil), ownership.InitialManagedKeys...)
	_, data, err := canonicalOMPModelProjectOwnership(ownership)
	return data, err
}

func journalContainsOriginLedger(journal *adapter.TransactionJournal, checksum string) bool {
	for _, entry := range journal.Entries {
		if filepath.ToSlash(entry.Path) == OMPModelProjectOwnershipRelativePath &&
			entry.MissingBefore && entry.AfterChecksum == checksum {
			return true
		}
	}
	return false
}

func validateOMPModelProjectOwnership(ownership ompModelProjectOwnership) error {
	if ownership.SchemaVersion != ompModelProjectOwnershipSchema {
		return fmt.Errorf("project ownership schema invalid")
	}
	if _, err := decodeOMPModelProjectSalt(ownership.Salt); err != nil {
		return err
	}
	for _, value := range []string{ownership.OriginalCommitment, ownership.InitialEmittedCommitment, ownership.LastEmittedCommitment} {
		if !validOMPModelHash(value) {
			return fmt.Errorf("project ownership commitment invalid")
		}
	}
	for _, keys := range [][]ompModelProjectOwnedKey{ownership.InitialManagedKeys, ownership.ManagedKeys} {
		seen := make(map[string]bool, len(keys))
		for _, key := range keys {
			if !validOMPManagedPath(key.Path) || !validOMPModelHash(key.Fingerprint) || seen[key.Path] {
				return fmt.Errorf("project ownership managed key invalid")
			}
			seen[key.Path] = true
		}
	}
	return nil
}

func decodeOMPModelProjectSalt(value string) ([]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("project ownership salt invalid")
	}
	return decoded, nil
}

func ompModelProjectCommitment(salt, data []byte, missing bool) string {
	marker := []byte("present\x00")
	if missing {
		marker = []byte("missing\x00")
	}
	payload := append(append(append([]byte("autopus.omp.project-commitment.v1\x00"), salt...), marker...), data...)
	return OMPModelSHA256(payload)
}

func ompModelProjectOwnedKeys(values map[string]string) []ompModelProjectOwnedKey {
	keys := make([]ompModelProjectOwnedKey, 0, len(values))
	for path, fingerprint := range values {
		keys = append(keys, ompModelProjectOwnedKey{Path: path, Fingerprint: fingerprint})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Path < keys[j].Path })
	return keys
}
