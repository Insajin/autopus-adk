package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func rollbackJournal(root string, journal *TransactionJournal) error {
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		if err := rollbackEntry(root, journal.Entries[i]); err != nil {
			return err
		}
	}
	journal.Status = TransactionStatusRolledBack
	return saveTransactionJournal(journal)
}

func ListCommittedTransactions(root string) ([]*TransactionJournal, error) {
	return loadTransactionJournals(root, "")
}

func RollbackTransactionJournal(root string, journal *TransactionJournal) error {
	return rollbackJournal(root, journal)
}

func rollbackEntry(root string, entry TransactionJournalEntry) error {
	_, abs, err := safeTransactionPath(root, entry.Path)
	if err != nil {
		return err
	}
	if entry.MissingBefore {
		if err := os.RemoveAll(abs); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rollback remove %s: %w", entry.Path, err)
		}
		return removeEmptyParents(root, filepath.Dir(abs))
	}
	if entry.BackupPath == "" {
		return fmt.Errorf("rollback backup missing for %s", entry.Path)
	}
	_, backupAbs, err := safeTransactionPath(root, entry.BackupPath)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rollback clear %s: %w", entry.Path, err)
	}
	if entry.Directory {
		return copyDir(backupAbs, abs)
	}
	return copyFile(backupAbs, abs, os.FileMode(entry.Mode))
}

func saveTransactionJournal(journal *TransactionJournal) error {
	if journal.Path == "" {
		return fmt.Errorf("transaction journal path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(journal.Path), 0755); err != nil {
		return fmt.Errorf("transaction journal dir: %w", err)
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("transaction journal encode: %w", err)
	}
	return os.WriteFile(journal.Path, append(data, '\n'), 0644)
}

func loadTransactionJournals(root, platform string) ([]*TransactionJournal, error) {
	dir := filepath.Join(root, manifestDir, "txns")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("transaction journal dir read: %w", err)
	}
	var journals []*TransactionJournal
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "journal.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var journal TransactionJournal
		if err := json.Unmarshal(data, &journal); err != nil {
			continue
		}
		if platform != "" && journal.Platform != platform {
			continue
		}
		if journal.Status != TransactionStatusCommitted {
			continue
		}
		journal.Path = path
		journals = append(journals, &journal)
	}
	return journals, nil
}

func safeTransactionPath(root, relPath string) (string, string, error) {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("unsafe transaction path %s", relPath)
	}
	return filepath.ToSlash(clean), filepath.Join(root, clean), nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		entryInfo, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}
		if entryInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath, entryInfo.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func TransactionWritesFromFiles(files []FileMapping, modeForPath func(string) os.FileMode) []TransactionWrite {
	writes := make([]TransactionWrite, 0, len(files))
	for _, file := range files {
		perm := os.FileMode(0644)
		if modeForPath != nil {
			perm = modeForPath(file.TargetPath)
		}
		writes = append(writes, TransactionWrite{
			Path:    file.TargetPath,
			Content: file.Content,
			Perm:    perm,
		})
	}
	return writes
}

func TransactionRemovesFromManifestDiff(diff ManifestDiff, recursive bool) []TransactionRemove {
	removes := make([]TransactionRemove, 0, len(diff.Prune))
	for _, entry := range diff.Prune {
		if entry.Action != ManifestActionPrune {
			continue
		}
		removes = append(removes, TransactionRemove{
			Path:      entry.Path,
			Recursive: recursive,
		})
	}
	return removes
}
