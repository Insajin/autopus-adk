package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

const workspaceConfigFileName = "autopus.yaml"

type appliedWorkspaceUpdate struct {
	Target         workspaceUpdateTarget
	Journals       []*adapter.TransactionJournal
	ConfigSnapshot *workspaceConfigSnapshot
}

type workspaceConfigSnapshot struct {
	data   []byte
	mode   os.FileMode
	exists bool
}

func captureWorkspaceConfigSnapshot(root string, preview bool) (*workspaceConfigSnapshot, error) {
	if preview {
		return nil, nil
	}
	path := filepath.Join(root, workspaceConfigFileName)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &workspaceConfigSnapshot{}, nil
		}
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &workspaceConfigSnapshot{data: data, mode: info.Mode().Perm(), exists: true}, nil
}

func restoreWorkspaceConfigSnapshot(root string, snapshot *workspaceConfigSnapshot) error {
	if snapshot == nil {
		return nil
	}
	path := filepath.Join(root, workspaceConfigFileName)
	if !snapshot.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, snapshot.data, snapshot.mode)
}

func committedTransactionSet(root string, preview bool) (map[string]bool, error) {
	if preview {
		return nil, nil
	}
	journals, err := adapter.ListCommittedTransactions(root)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(journals))
	for _, journal := range journals {
		seen[transactionJournalKey(journal)] = true
	}
	return seen, nil
}

func newCommittedTransactions(root string, before map[string]bool, preview bool) ([]*adapter.TransactionJournal, error) {
	if preview {
		return nil, nil
	}
	journals, err := adapter.ListCommittedTransactions(root)
	if err != nil {
		return nil, err
	}
	var next []*adapter.TransactionJournal
	for _, journal := range journals {
		if !before[transactionJournalKey(journal)] {
			next = append(next, journal)
		}
	}
	return next, nil
}

func transactionJournalKey(journal *adapter.TransactionJournal) string {
	if journal == nil {
		return ""
	}
	if journal.Path != "" {
		return journal.Path
	}
	return journal.Platform + ":" + journal.ID
}

func rollbackWorkspaceUpdateWithCurrent(applied []appliedWorkspaceUpdate, current appliedWorkspaceUpdate) error {
	rollbackSet := append(append([]appliedWorkspaceUpdate(nil), applied...), current)
	return rollbackAppliedWorkspaceUpdates(rollbackSet)
}

func rollbackAppliedWorkspaceUpdates(applied []appliedWorkspaceUpdate) error {
	var errs []error
	for i := len(applied) - 1; i >= 0; i-- {
		item := applied[i]
		for j := len(item.Journals) - 1; j >= 0; j-- {
			if err := adapter.RollbackTransactionJournal(item.Target.AbsPath, item.Journals[j]); err != nil {
				errs = append(errs, fmt.Errorf("%s rollback failed: %w", item.Target.Path, err))
			}
		}
		if err := restoreWorkspaceConfigSnapshot(item.Target.AbsPath, item.ConfigSnapshot); err != nil {
			errs = append(errs, fmt.Errorf("%s config rollback failed: %w", item.Target.Path, err))
		}
	}
	return errors.Join(errs...)
}
