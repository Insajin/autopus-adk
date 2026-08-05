package omp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (tx *ompRootedTransaction) rollback() error {
	for index := len(tx.journal.Entries) - 1; index >= 0; index-- {
		entry := tx.journal.Entries[index]
		if entry.MissingBefore {
			if err := tx.workspace.remove(entry.Path, true); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("rollback remove %s: %w", entry.Path, err)
			}
			_ = tx.workspace.removeEmptyParents(filepath.Dir(entry.Path))
			continue
		}
		if filepath.ToSlash(entry.Path) == configFile {
			preimage, ok := tx.configPreimages[entry.Path]
			if !ok {
				return fmt.Errorf("rollback config preimage missing for %s", entry.Path)
			}
			if err := tx.workspace.atomicWrite(entry.Path, preimage.data, preimage.mode); err != nil {
				return err
			}
			if entry.BackupPath != "" {
				if err := tx.workspace.remove(entry.BackupPath, false); err != nil &&
					!errors.Is(err, fs.ErrNotExist) {
					return err
				}
				_ = tx.workspace.removeEmptyParents(filepath.Dir(entry.BackupPath))
			}
			continue
		}
		if entry.BackupPath == "" || entry.Directory {
			return fmt.Errorf("rollback backup missing for %s", entry.Path)
		}
		data, _, err := tx.workspace.readOwnerOnlyFile(entry.BackupPath, 0)
		if err != nil {
			return err
		}
		if err := tx.workspace.atomicWrite(entry.Path, data, os.FileMode(entry.Mode)); err != nil {
			return err
		}
		if err := tx.workspace.remove(entry.BackupPath, false); err != nil &&
			!errors.Is(err, fs.ErrNotExist) {
			return err
		}
		_ = tx.workspace.removeEmptyParents(filepath.Dir(entry.BackupPath))
	}
	tx.journal.Status = adapter.TransactionStatusRolledBack
	return tx.saveJournal()
}
