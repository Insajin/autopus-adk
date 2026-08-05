package omp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOMPModelProjectOwnershipAt_MissingInvalidAndCanonical(t *testing.T) {
	t.Parallel()

	withWorkspace := func(t *testing.T, prepare func(string)) *ompRootedWorkspace {
		t.Helper()
		root := t.TempDir()
		if prepare != nil {
			prepare(root)
		}
		workspace, err := openOMPRootedWorkspace(root)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, workspace.Close()) })
		return workspace
	}

	t.Run("missing", func(t *testing.T) {
		ownership, exists, err := readOMPModelProjectOwnershipAt(withWorkspace(t, nil))
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Empty(t, ownership.LedgerDigest)
	})
	t.Run("directory instead of ledger", func(t *testing.T) {
		workspace := withWorkspace(t, func(root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, OMPModelProjectOwnershipRelativePath), 0o700))
		})
		_, exists, err := readOMPModelProjectOwnershipAt(workspace)
		assert.False(t, exists)
		assert.ErrorContains(t, err, "read project ownership ledger")
	})
	t.Run("malformed ledger", func(t *testing.T) {
		workspace := withWorkspace(t, func(root string) {
			writeOMPOwnershipCoverageFile(t, root, OMPModelProjectOwnershipRelativePath, []byte("{}\n{}\n"), 0o600)
		})
		_, exists, err := readOMPModelProjectOwnershipAt(workspace)
		assert.False(t, exists)
		assert.ErrorContains(t, err, "ledger invalid")
	})
	t.Run("non-owner-only ledger", func(t *testing.T) {
		ownership, data, err := newOMPModelProjectOwnership(
			nil, true, []byte("emitted"), map[string]string{},
		)
		require.NoError(t, err)
		require.NotEmpty(t, ownership.LedgerDigest)
		workspace := withWorkspace(t, func(root string) {
			writeOMPOwnershipCoverageFile(
				t, root, OMPModelProjectOwnershipRelativePath, data, 0o644,
			)
		})
		_, exists, err := readOMPModelProjectOwnershipAt(workspace)
		assert.False(t, exists)
		assert.ErrorContains(t, err, "mode 0600")
	})
}

func TestReadOMPModelProjectConfigAt_MissingAndUnsafeTypes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	data, missing, mode, err := readOMPModelProjectConfigAt(workspace)
	require.NoError(t, err)
	assert.True(t, missing)
	assert.Nil(t, data)
	assert.Zero(t, mode)

	require.NoError(t, os.MkdirAll(filepath.Join(root, configFile), 0o700))
	data, missing, mode, err = readOMPModelProjectConfigAt(workspace)
	assert.ErrorContains(t, err, "read "+configFile)
	assert.False(t, missing)
	assert.Nil(t, data)
	assert.Zero(t, mode)
}

func TestValidateCurrentOMPModelProjectConfigAt_RejectsMissingConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })
	ownership, _, err := newOMPModelProjectOwnership(nil, true, []byte("emitted"), map[string]string{})
	require.NoError(t, err)

	data, err := validateCurrentOMPModelProjectConfigAt(workspace, ownership)
	assert.Nil(t, data)
	assert.ErrorContains(t, err, "current project config is unavailable")
}

func TestLoadOMPTransactionJournalsAt_FiltersUnsafeOrIrrelevantEntries(t *testing.T) {
	t.Parallel()

	t.Run("missing directory", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
		require.NoError(t, err)
		assert.Empty(t, journals)
	})
	t.Run("path is a file", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		require.NoError(t, workspace.atomicWrite(".autopus/txns", []byte("blocked"), 0o600))
		journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
		assert.Nil(t, journals)
		assert.ErrorContains(t, err, "transaction journal dir read")
	})
	t.Run("filters non-directory malformed pending and other platform", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		require.NoError(t, workspace.atomicWrite(".autopus/txns/plain", []byte("file"), 0o600))
		require.NoError(t, workspace.atomicWrite(".autopus/txns/malformed/journal.json", []byte("{"), 0o600))
		writeOMPTransactionJournalForCoverage(t, workspace, "pending", &adapter.TransactionJournal{
			Platform: adapterName, Status: adapter.TransactionStatusPending,
		})
		writeOMPTransactionJournalForCoverage(t, workspace, "other", &adapter.TransactionJournal{
			Platform: "codex", Status: adapter.TransactionStatusCommitted,
		})
		journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
		require.NoError(t, err)
		assert.Empty(t, journals)
	})
	t.Run("skips directories without journals", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		root, err := workspace.openDir(".autopus/txns/no-journal", true, 0o700)
		require.NoError(t, err)
		require.NoError(t, root.Close())
		journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
		require.NoError(t, err)
		assert.Empty(t, journals)
	})
	t.Run("sorts committed journals newest first", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		writeOMPTransactionJournalForCoverage(t, workspace, "older", &adapter.TransactionJournal{
			ID: "older", Platform: adapterName, Status: adapter.TransactionStatusCommitted,
			CreatedAt: "2026-08-01T00:00:00Z",
		})
		writeOMPTransactionJournalForCoverage(t, workspace, "newer", &adapter.TransactionJournal{
			ID: "newer", Platform: adapterName, Status: adapter.TransactionStatusCommitted,
			CreatedAt: "2026-08-02T00:00:00Z",
		})
		journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
		require.NoError(t, err)
		require.Len(t, journals, 2)
		assert.Equal(t, "newer", journals[0].ID)
		assert.Equal(t, "older", journals[1].ID)
		assert.False(t, filepath.IsAbs(journals[0].Path))
		assert.Equal(t, filepath.ToSlash(journals[0].Path), journals[0].Path)
	})
}

func TestLoadOMPModelProjectOriginalPreimageAt_ValidatesOriginEvidence(t *testing.T) {
	t.Parallel()

	t.Run("invalid ownership", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ompModelProjectOwnership{Salt: "bad"})
		assert.ErrorContains(t, err, "salt invalid")
	})
	t.Run("origin transaction missing", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, false, []byte("original"), []byte("emitted"))
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		assert.ErrorContains(t, err, "origin transaction missing")
	})
	t.Run("journal directory error is surfaced", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, false, []byte("original"), []byte("emitted"))
		require.NoError(t, workspace.atomicWrite(".autopus/txns", []byte("blocked"), 0o600))
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		assert.ErrorContains(t, err, "load project ownership transaction")
	})
	t.Run("unrelated committed journal is ignored", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, false, []byte("original"), []byte("emitted"))
		writeOMPTransactionJournalForCoverage(t, workspace, "unrelated", &adapter.TransactionJournal{
			Platform: adapterName, Status: adapter.TransactionStatusCommitted,
			Entries: []adapter.TransactionJournalEntry{{Path: "other", AfterChecksum: "different"}},
		})
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		assert.ErrorContains(t, err, "origin transaction missing")
	})
	t.Run("missing preimage contradicts ledger", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, false, []byte("original"), []byte("emitted"))
		writeOMPOriginJournalForCoverage(t, workspace, ownership, adapter.TransactionJournalEntry{
			Path: configFile, MissingBefore: true,
		})
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		assert.ErrorContains(t, err, "preimage mismatch")
	})
	t.Run("originally missing is observable", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, true, nil, []byte("emitted"))
		writeOMPOriginJournalForCoverage(t, workspace, ownership, adapter.TransactionJournalEntry{
			Path: configFile, MissingBefore: true,
		})
		preimage, missing, mode, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		require.NoError(t, err)
		assert.True(t, missing)
		assert.Nil(t, preimage)
		assert.Zero(t, mode)
	})
	t.Run("missing backup fails closed", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, false, []byte("original"), []byte("emitted"))
		writeOMPOriginJournalForCoverage(t, workspace, ownership, adapter.TransactionJournalEntry{
			Path: configFile, BackupPath: ".autopus/backup/missing", Mode: 0o640,
		})
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		assert.ErrorContains(t, err, "read project ownership preimage")
	})
	t.Run("wrong backup commitment fails closed", func(t *testing.T) {
		workspace := openOMPWorkspaceForCoverage(t)
		ownership := newOMPProjectOwnershipForCoverage(t, false, []byte("original"), []byte("emitted"))
		backup := ".autopus/backup/wrong"
		require.NoError(t, workspace.atomicWrite(backup, []byte("not-original"), 0o640))
		writeOMPOriginJournalForCoverage(t, workspace, ownership, adapter.TransactionJournalEntry{
			Path: configFile, BackupPath: backup, Mode: 0o640,
		})
		_, _, _, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		assert.ErrorContains(t, err, "preimage mismatch")
	})
}

func TestOMPModelProjectOwnership_PureValidationBranches(t *testing.T) {
	t.Parallel()

	_, _, err := updateOMPModelProjectOwnership(ompModelProjectOwnership{Salt: "bad"}, nil, nil)
	assert.ErrorContains(t, err, "salt invalid")
	_, _, err = canonicalOMPModelProjectOwnership(ompModelProjectOwnership{Salt: "bad"})
	assert.Error(t, err)

	missing, _, err := newOMPModelProjectOwnership(nil, true, []byte("emitted"), map[string]string{})
	require.NoError(t, err)
	present, _, err := newOMPModelProjectOwnership(nil, false, []byte("emitted"), map[string]string{})
	require.NoError(t, err)
	assert.NotEqual(t, missing.OriginalCommitment, present.OriginalCommitment)

	badHash := "sha256:" + strings.Repeat("0", 64)
	validationCases := []ompModelProjectOwnership{
		{SchemaVersion: "wrong"},
		{SchemaVersion: ompModelProjectOwnershipSchema, Salt: "bad"},
		{SchemaVersion: ompModelProjectOwnershipSchema, Salt: strings.Repeat("00", 32)},
		{SchemaVersion: ompModelProjectOwnershipSchema, Salt: strings.Repeat("00", 32), OriginalCommitment: badHash, InitialEmittedCommitment: badHash, LastEmittedCommitment: badHash, ManagedKeys: []ompModelProjectOwnedKey{{Path: "../bad", Fingerprint: badHash}}},
	}
	for _, ownership := range validationCases {
		assert.Error(t, validateOMPModelProjectOwnership(ownership))
	}
	_, err = decodeOMPModelProjectSalt("xyz")
	assert.ErrorContains(t, err, "salt invalid")
}
