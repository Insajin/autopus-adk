package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func readOMPModelProjectOwnershipAt(workspace *ompRootedWorkspace) (ompModelProjectOwnership, bool, error) {
	data, _, err := workspace.readFile(OMPModelProjectOwnershipRelativePath, 4<<20)
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
	for _, journal := range journals {
		if !journalContainsOriginLedger(journal, wantLedgerChecksum) {
			continue
		}
		for _, entry := range journal.Entries {
			if filepath.ToSlash(entry.Path) != configFile {
				continue
			}
			if entry.MissingBefore {
				if !ownership.OriginalMissing {
					return nil, false, 0, fmt.Errorf("project ownership preimage mismatch")
				}
				return nil, true, 0, nil
			}
			preimage, _, readErr := workspace.readFile(entry.BackupPath, 4<<20)
			if readErr != nil {
				return nil, false, 0, fmt.Errorf("read project ownership preimage: %w", readErr)
			}
			salt, saltErr := decodeOMPModelProjectSalt(ownership.Salt)
			if saltErr != nil || ompModelProjectCommitment(salt, preimage, false) != ownership.OriginalCommitment {
				return nil, false, 0, fmt.Errorf("project ownership preimage mismatch")
			}
			return preimage, false, fs.FileMode(entry.Mode), nil
		}
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
		data, _, readErr := workspace.readFile(rel, 4<<20)
		if readErr != nil {
			continue
		}
		var journal adapter.TransactionJournal
		if json.Unmarshal(data, &journal) != nil || journal.Status != adapter.TransactionStatusCommitted ||
			(platform != "" && journal.Platform != platform) {
			continue
		}
		journal.Path, _ = workspace.absolute(rel)
		journals = append(journals, &journal)
	}
	sort.Slice(journals, func(i, j int) bool { return journals[i].CreatedAt > journals[j].CreatedAt })
	return journals, nil
}
