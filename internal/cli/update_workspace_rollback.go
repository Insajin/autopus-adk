package cli

import (
	"errors"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

type appliedWorkspaceUpdate struct {
	Target   workspaceUpdateTarget
	Journals []*adapter.TransactionJournal
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

func rollbackAppliedWorkspaceUpdates(applied []appliedWorkspaceUpdate) error {
	var errs []error
	for i := len(applied) - 1; i >= 0; i-- {
		item := applied[i]
		for j := len(item.Journals) - 1; j >= 0; j-- {
			if err := adapter.RollbackTransactionJournal(item.Target.AbsPath, item.Journals[j]); err != nil {
				errs = append(errs, fmt.Errorf("%s rollback failed: %w", item.Target.Path, err))
			}
		}
	}
	return errors.Join(errs...)
}
