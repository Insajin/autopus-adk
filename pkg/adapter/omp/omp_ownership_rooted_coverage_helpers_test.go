package omp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/require"
)

func openOMPWorkspaceForCoverage(t *testing.T) *ompRootedWorkspace {
	t.Helper()
	workspace, err := openOMPRootedWorkspace(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })
	return workspace
}

func newOMPProjectOwnershipForCoverage(t *testing.T, missing bool, original, emitted []byte) ompModelProjectOwnership {
	t.Helper()
	ownership, _, err := newOMPModelProjectOwnership(original, missing, emitted, map[string]string{})
	require.NoError(t, err)
	return ownership
}

func writeOMPTransactionJournalForCoverage(t *testing.T, workspace *ompRootedWorkspace, id string, journal *adapter.TransactionJournal) {
	t.Helper()
	data, err := json.Marshal(journal)
	require.NoError(t, err)
	require.NoError(t, workspace.atomicWrite(filepath.Join(".autopus", "txns", id, "journal.json"), data, 0o600))
}

func writeOMPOriginJournalForCoverage(t *testing.T, workspace *ompRootedWorkspace, ownership ompModelProjectOwnership, configEntry adapter.TransactionJournalEntry) {
	t.Helper()
	originData, err := ompModelProjectOriginLedgerBytes(ownership)
	require.NoError(t, err)
	require.NoError(t, workspace.atomicWrite(configFile, []byte("emitted"), 0o600))
	configEntry.AfterChecksum = adapter.Checksum("emitted")
	writeOMPTransactionJournalForCoverage(t, workspace, "origin", &adapter.TransactionJournal{
		Platform: adapterName, Status: adapter.TransactionStatusCommitted, CreatedAt: "2026-08-02T00:00:00Z",
		Entries: []adapter.TransactionJournalEntry{
			{Path: OMPModelProjectOwnershipRelativePath, MissingBefore: true, AfterChecksum: adapter.Checksum(string(originData))},
			configEntry,
		},
	})
}

func writeOMPOwnershipCoverageFile(t *testing.T, root, relative string, data []byte, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, data, mode))
}
