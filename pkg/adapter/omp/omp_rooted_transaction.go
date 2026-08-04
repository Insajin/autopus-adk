package omp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

type ompRootedTransaction struct {
	workspace  *ompRootedWorkspace
	platform   string
	id         string
	journalRel string
	backupRel  string
	journal    *adapter.TransactionJournal
	snapshots  map[string]bool
}

func applyOMPTransactionAt(
	workspace *ompRootedWorkspace,
	platform string,
	plan adapter.TransactionPlan,
) (*adapter.TransactionJournal, error) {
	if err := validateOMPRootedTransactionPlan(plan); err != nil {
		return nil, err
	}
	tx, err := newOMPRootedTransaction(workspace, platform)
	if err != nil {
		return nil, err
	}
	if err := tx.apply(plan); err != nil {
		if rollbackErr := tx.rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return nil, err
	}
	tx.journal.Status = adapter.TransactionStatusCommitted
	if err := tx.saveJournal(); err != nil {
		return nil, err
	}
	return tx.journal, nil
}

func validateOMPRootedTransactionPlan(plan adapter.TransactionPlan) error {
	paths := []string{filepath.Join(".autopus", "txns", ".guard"), filepath.Join(".autopus", "backup", ".guard")}
	for _, write := range plan.Writes {
		paths = append(paths, write.Path)
	}
	for _, remove := range plan.Removes {
		paths = append(paths, remove.Path)
	}
	if plan.Manifest != nil {
		paths = append(paths, filepath.Join(".autopus", plan.Manifest.Platform+"-manifest.json"))
	}
	for _, path := range paths {
		if _, err := cleanOMPRootedPath(path); err != nil {
			return err
		}
	}
	return nil
}

func newOMPRootedTransaction(workspace *ompRootedWorkspace, platform string) (*ompRootedTransaction, error) {
	id := time.Now().UTC().Format("20060102T150405.000000000")
	safePlatform := strings.NewReplacer("/", "-", string(os.PathSeparator), "-").Replace(platform)
	dirRel := filepath.Join(".autopus", "txns", id+"-"+safePlatform)
	root, err := workspace.openDir(dirRel, true, 0o755)
	if err != nil {
		return nil, fmt.Errorf("transaction dir: %w", err)
	}
	_ = root.Close()
	journalRel := filepath.Join(dirRel, "journal.json")
	journalPath, err := workspace.absolute(journalRel)
	if err != nil {
		return nil, err
	}
	tx := &ompRootedTransaction{
		workspace: workspace, platform: platform, id: id, journalRel: journalRel,
		backupRel: filepath.Join(".autopus", "backup", id, "transaction", safePlatform),
		journal: &adapter.TransactionJournal{ID: id, Platform: platform,
			Status: adapter.TransactionStatusPending, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Path: journalPath},
		snapshots: make(map[string]bool),
	}
	return tx, tx.saveJournal()
}

func (tx *ompRootedTransaction) apply(plan adapter.TransactionPlan) error {
	for _, remove := range plan.Removes {
		if err := tx.removePath(remove); err != nil {
			return err
		}
	}
	for _, write := range plan.Writes {
		if err := tx.writeFile(write); err != nil {
			return err
		}
	}
	if plan.Manifest != nil {
		return tx.writeManifest(plan.Manifest)
	}
	return nil
}

func (tx *ompRootedTransaction) writeFile(write adapter.TransactionWrite) error {
	rel, err := cleanOMPRootedPath(write.Path)
	if err != nil {
		return err
	}
	if write.Perm == 0 {
		write.Perm = 0o644
	}
	if err := tx.snapshot(rel, "write"); err != nil {
		return err
	}
	if info, statErr := tx.workspace.lstat(rel); statErr == nil && info.IsDir() {
		return fmt.Errorf("transaction target is directory %s", rel)
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("transaction stat %s: %w", rel, statErr)
	}
	if err := tx.workspace.atomicWrite(rel, write.Content, write.Perm); err != nil {
		return fmt.Errorf("transaction replace %s: %w", rel, err)
	}
	tx.markAfterChecksum(rel, adapter.Checksum(string(write.Content)))
	return tx.saveJournal()
}

func (tx *ompRootedTransaction) removePath(remove adapter.TransactionRemove) error {
	rel, err := cleanOMPRootedPath(remove.Path)
	if err != nil {
		return err
	}
	if _, err := tx.workspace.lstat(rel); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := tx.snapshot(rel, "remove"); err != nil {
		return err
	}
	if err := tx.workspace.remove(rel, remove.Recursive); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("transaction remove %s: %w", rel, err)
	}
	return tx.workspace.removeEmptyParents(filepath.Dir(rel))
}

func (tx *ompRootedTransaction) writeManifest(manifest *adapter.Manifest) error {
	manifest.GeneratedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest serialize: %w", err)
	}
	return tx.writeFile(adapter.TransactionWrite{
		Path:    filepath.Join(".autopus", manifest.Platform+"-manifest.json"),
		Content: append(data, '\n'), Perm: 0o600,
	})
}

func (tx *ompRootedTransaction) snapshot(rel, operation string) error {
	if tx.snapshots[rel] {
		return nil
	}
	entry := adapter.TransactionJournalEntry{Path: filepath.ToSlash(rel), Operation: operation}
	info, err := tx.workspace.lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		entry.MissingBefore = true
		tx.addSnapshot(entry)
		return tx.saveJournal()
	}
	if err != nil {
		return fmt.Errorf("transaction stat %s: %w", rel, err)
	}
	if info.IsDir() {
		return fmt.Errorf("transaction directory snapshots are unsupported for %s", rel)
	}
	entry.Mode = uint32(info.Mode().Perm())
	backupRel := filepath.Join(tx.backupRel, rel)
	if err := tx.workspace.copyFile(rel, backupRel, info.Mode().Perm()); err != nil {
		return fmt.Errorf("transaction backup %s: %w", rel, err)
	}
	entry.BackupPath = filepath.ToSlash(backupRel)
	tx.addSnapshot(entry)
	return tx.saveJournal()
}

func (tx *ompRootedTransaction) addSnapshot(entry adapter.TransactionJournalEntry) {
	tx.snapshots[entry.Path] = true
	tx.journal.Entries = append(tx.journal.Entries, entry)
}

func (tx *ompRootedTransaction) markAfterChecksum(rel, checksum string) {
	for index := range tx.journal.Entries {
		if tx.journal.Entries[index].Path == filepath.ToSlash(rel) {
			tx.journal.Entries[index].AfterChecksum = checksum
			return
		}
	}
}

func (tx *ompRootedTransaction) saveJournal() error {
	data, err := json.MarshalIndent(tx.journal, "", "  ")
	if err != nil {
		return fmt.Errorf("transaction journal encode: %w", err)
	}
	return tx.workspace.atomicWrite(tx.journalRel, append(data, '\n'), 0o600)
}

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
		if entry.BackupPath == "" || entry.Directory {
			return fmt.Errorf("rollback backup missing for %s", entry.Path)
		}
		data, _, err := tx.workspace.readFile(entry.BackupPath, 0)
		if err != nil {
			return err
		}
		if err := tx.workspace.atomicWrite(entry.Path, data, os.FileMode(entry.Mode)); err != nil {
			return err
		}
	}
	tx.journal.Status = adapter.TransactionStatusRolledBack
	return tx.saveJournal()
}

func loadOMPManifestAt(workspace *ompRootedWorkspace, platform string) (*adapter.Manifest, error) {
	path := filepath.Join(".autopus", platform+"-manifest.json")
	data, _, err := workspace.readFile(path, 4<<20)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("매니페스트 읽기 실패: %w", err)
	}
	var manifest adapter.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("매니페스트 파싱 실패: %w", err)
	}
	return &manifest, nil
}
