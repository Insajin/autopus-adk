package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func readOMPModelProjectOwnershipAt(workspace *ompRootedWorkspace) (ompModelProjectOwnership, bool, error) {
	data, _, err := workspace.readOwnerOnlyFile(OMPModelProjectOwnershipRelativePath, 4<<20)
	if errors.Is(err, fs.ErrNotExist) {
		return ompModelProjectOwnership{}, false, nil
	}
	if err != nil {
		return ompModelProjectOwnership{}, false, fmt.Errorf("read project ownership ledger: %w", err)
	}
	var ownership ompModelProjectOwnership
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ownership); err != nil || requireOMPModelDoctorJSONEOF(decoder) != nil {
		return ompModelProjectOwnership{}, false, fmt.Errorf("project ownership ledger invalid")
	}
	wantDigest := ownership.LedgerDigest
	canonical, _, err := canonicalOMPModelProjectOwnership(ownership)
	if err != nil || canonical.LedgerDigest != wantDigest {
		return ompModelProjectOwnership{}, false, fmt.Errorf("project ownership ledger invalid")
	}
	return canonical, true, nil
}

func readOMPModelProjectConfigAt(workspace *ompRootedWorkspace) ([]byte, bool, fs.FileMode, error) {
	data, info, err := workspace.readFile(configFile, 4<<20)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, true, 0, nil
	}
	if err != nil {
		return nil, false, 0, fmt.Errorf("read %s: %w", configFile, err)
	}
	return data, false, info.Mode().Perm(), nil
}

func validateCurrentOMPModelProjectConfigAt(
	workspace *ompRootedWorkspace,
	ownership ompModelProjectOwnership,
) ([]byte, error) {
	current, missing, _, err := readOMPModelProjectConfigAt(workspace)
	if err != nil || missing {
		return nil, fmt.Errorf("managed_key_conflict: current project config is unavailable")
	}
	salt, err := decodeOMPModelProjectSalt(ownership.Salt)
	if err != nil || ompModelProjectCommitment(salt, current, false) != ownership.LastEmittedCommitment {
		return nil, fmt.Errorf("managed_key_conflict: current project config differs from last emitted bytes")
	}
	return current, nil
}

func loadOMPModelProjectOriginalPreimageAt(
	workspace *ompRootedWorkspace,
	ownership ompModelProjectOwnership,
) ([]byte, bool, fs.FileMode, error) {
	originData, err := ompModelProjectOriginLedgerBytes(ownership)
	if err != nil {
		return nil, false, 0, err
	}
	journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
	if err != nil {
		return nil, false, 0, fmt.Errorf("load project ownership transaction: %w", err)
	}
	wantLedgerChecksum := adapter.Checksum(string(originData))
	hasOrigin := false
	for _, journal := range journals {
		hasOrigin = hasOrigin || journalContainsOriginLedger(journal, wantLedgerChecksum)
	}
	if !hasOrigin {
		return nil, false, 0, fmt.Errorf("project ownership origin transaction missing")
	}
	current, err := validateCurrentOMPModelProjectConfigAt(workspace, ownership)
	if err != nil {
		return nil, false, 0, err
	}
	salt, err := decodeOMPModelProjectSalt(ownership.Salt)
	if err != nil {
		return nil, false, 0, err
	}
	for _, journal := range journals {
		var configEntry *adapter.TransactionJournalEntry
		currentChecksum := adapter.Checksum(string(current))
		for index := range journal.Entries {
			entry := &journal.Entries[index]
			if filepath.ToSlash(entry.Path) == configFile && entry.AfterChecksum == currentChecksum {
				configEntry = entry
				break
			}
		}
		if configEntry == nil {
			continue
		}
		origin := journalContainsOriginLedger(journal, wantLedgerChecksum)
		if configEntry.MissingBefore {
			if !origin || !ownership.OriginalMissing {
				return nil, false, 0, fmt.Errorf("project ownership preimage mismatch")
			}
			return nil, true, 0, nil
		}
		if configEntry.BackupPath == "" {
			return nil, false, 0, fmt.Errorf("read project ownership preimage: rollback artifact missing")
		}
		backup, info, readErr := workspace.readFile(
			configEntry.BackupPath, maxOMPConfigRollbackArtifact,
		)
		if readErr != nil {
			return nil, false, 0, fmt.Errorf("read project ownership preimage: %w", readErr)
		}
		artifact, artifactErr := decodeOMPConfigRollbackArtifact(backup)
		var before []byte
		if artifactErr == nil {
			if info.Mode().Perm() != 0o600 || artifact.AfterChecksum != configEntry.AfterChecksum {
				return nil, false, 0, fmt.Errorf("project ownership rollback artifact invalid")
			}
			before, err = applyOMPConfigRollbackArtifact(artifact, current)
		} else {
			before = append([]byte(nil), backup...)
			migrated, migrationErr := encodeOMPConfigRollbackArtifact(before, current)
			if migrationErr != nil {
				return nil, false, 0, fmt.Errorf("migrate project ownership preimage: %w", migrationErr)
			}
			if migrationErr = workspace.atomicWrite(configEntry.BackupPath, migrated, 0o600); migrationErr != nil {
				return nil, false, 0, fmt.Errorf("migrate project ownership preimage: %w", migrationErr)
			}
		}
		if err != nil {
			return nil, false, 0, fmt.Errorf("read project ownership preimage: %w", err)
		}
		current = before
		if !origin {
			continue
		}
		if ownership.OriginalMissing ||
			ompModelProjectCommitment(salt, current, false) != ownership.OriginalCommitment {
			return nil, false, 0, fmt.Errorf("project ownership preimage mismatch")
		}
		return current, false, fs.FileMode(configEntry.Mode), nil
	}
	return nil, false, 0, fmt.Errorf("project ownership origin transaction missing")
}

func loadOMPTransactionJournalsAt(
	workspace *ompRootedWorkspace,
	platform string,
) ([]*adapter.TransactionJournal, error) {
	entries, err := workspace.readDir(filepath.Join(".autopus", "txns"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("transaction journal dir read: %w", err)
	}
	var journals []*adapter.TransactionJournal
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.Join(".autopus", "txns", entry.Name(), "journal.json")
		data, _, readErr := workspace.readOwnerOnlyFile(rel, 4<<20)
		if readErr != nil {
			continue
		}
		var journal adapter.TransactionJournal
		if json.Unmarshal(data, &journal) != nil || journal.Status != adapter.TransactionStatusCommitted ||
			(platform != "" && journal.Platform != platform) {
			continue
		}
		journal.Path = filepath.ToSlash(rel)
		journals = append(journals, &journal)
	}

	sort.Slice(journals, func(i, j int) bool { return journals[i].CreatedAt > journals[j].CreatedAt })
	return journals, nil
}

func (workspace *ompRootedWorkspace) cleanupOMPConfigRollbackArtifactsIfUnowned() error {
	if _, err := workspace.lstat(OMPModelProjectOwnershipRelativePath); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	entries, err := workspace.readDir(filepath.Join(".autopus", "txns"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return nil
	}
	var cleanupErr error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		journalRel := filepath.Join(".autopus", "txns", entry.Name(), "journal.json")
		data, _, readErr := workspace.readOwnerOnlyFile(journalRel, 4<<20)
		if readErr != nil {
			continue
		}
		var journal adapter.TransactionJournal
		if json.Unmarshal(data, &journal) != nil {
			continue
		}
		for _, journalEntry := range journal.Entries {
			if filepath.ToSlash(journalEntry.Path) != configFile || journalEntry.BackupPath == "" {
				continue
			}
			backup, pathErr := cleanOMPRootedPath(journalEntry.BackupPath)
			if pathErr != nil ||
				!strings.HasPrefix(filepath.ToSlash(backup), ".autopus/backup/") {
				continue
			}
			if removeErr := workspace.remove(backup, false); removeErr != nil &&
				!errors.Is(removeErr, fs.ErrNotExist) {
				cleanupErr = errors.Join(cleanupErr, removeErr)
				continue
			}
			if parentErr := workspace.removeEmptyParents(filepath.Dir(backup)); parentErr != nil {
				cleanupErr = errors.Join(cleanupErr, parentErr)
			}
		}
	}
	return cleanupErr
}
