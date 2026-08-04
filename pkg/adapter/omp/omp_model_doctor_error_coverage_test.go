package omp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOMPModelDoctorReceipt_RejectsOversizeDigestDriftAndSymlink(t *testing.T) {
	t.Parallel()

	t.Run("missing receipt under real parent", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, ".autopus"), 0o700))
		_, reason := readOMPModelDoctorReceipt(root)
		assert.Equal(t, "receipt_missing", reason)
	})

	t.Run("oversized", func(t *testing.T) {
		root := t.TempDir()
		writeOMPDoctorErrorFile(t, root, OMPModelReceiptRelativePath,
			bytes.Repeat([]byte(" "), maxOMPModelDoctorReceiptBytes+1), 0o600)
		_, reason := readOMPModelDoctorReceipt(root)
		assert.Equal(t, "receipt_invalid", reason)
	})

	t.Run("resolution digest drift", func(t *testing.T) {
		root := t.TempDir()
		receipt, _, err := CanonicalOMPModelResolutionReceipt(
			modelReceiptFixture(time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)),
		)
		require.NoError(t, err)
		receipt.ResolutionDigest = doctorHash("d")
		data, err := json.MarshalIndent(receipt, "", "  ")
		require.NoError(t, err)
		writeOMPDoctorErrorFile(t, root, OMPModelReceiptRelativePath, append(data, '\n'), 0o600)
		_, reason := readOMPModelDoctorReceipt(root)
		assert.Equal(t, "receipt_invalid", reason)
	})

	t.Run("invalid trailing JSON", func(t *testing.T) {
		root := t.TempDir()
		writeOMPDoctorErrorReceipt(t, root)
		path := filepath.Join(root, OMPModelReceiptRelativePath)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, append(data, '{'), 0o600))
		_, reason := readOMPModelDoctorReceipt(root)
		assert.Equal(t, "receipt_invalid", reason)
	})

	t.Run("decodable but invalid receipt", func(t *testing.T) {
		root := t.TempDir()
		writeOMPDoctorErrorFile(t, root, OMPModelReceiptRelativePath, []byte("{}\n"), 0o600)
		_, reason := readOMPModelDoctorReceipt(root)
		assert.Equal(t, "receipt_invalid", reason)
	})

	t.Run("symlinked ownership directory", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		writeOMPDoctorErrorReceipt(t, outside)
		require.NoError(t, os.Symlink(filepath.Join(outside, ".autopus"), filepath.Join(root, ".autopus")))
		_, reason := readOMPModelDoctorReceipt(root)
		assert.Equal(t, "receipt_invalid", reason)
	})
}

func TestReadOMPModelDoctorReceiptAt_RejectsCanonicalDigestDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	receipt, _, err := CanonicalOMPModelResolutionReceipt(
		modelReceiptFixture(time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)),
	)
	require.NoError(t, err)
	receipt.ResolutionDigest = doctorHash("e")
	data, err := json.MarshalIndent(receipt, "", "  ")
	require.NoError(t, err)
	writeOMPDoctorErrorFile(t, root, OMPModelReceiptRelativePath, append(data, '\n'), 0o600)
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	_, reason := readOMPModelDoctorReceiptAt(workspace)
	assert.Equal(t, "receipt_invalid", reason)
}

func TestOMPModelDoctorProjectionMatches_RejectsAmbiguousAgentIdentity(t *testing.T) {
	t.Parallel()

	input := modelDoctorInput(t.TempDir())
	receipt, _, err := CanonicalOMPModelResolutionReceipt(
		modelReceiptFixture(time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)),
	)
	require.NoError(t, err)

	t.Run("empty route identity", func(t *testing.T) {
		current := input
		current.Compilation.Resolutions = append([]OMPModelRouteResolution(nil), input.Compilation.Resolutions...)
		current.Compilation.Resolutions[0].Agent = ""
		current.Compilation.Resolutions[0].RouteID = ""
		assert.False(t, ompModelDoctorProjectionMatches(receipt, current))
	})

	t.Run("duplicate agent identity", func(t *testing.T) {
		current := input
		current.Compilation.Resolutions = append([]OMPModelRouteResolution(nil), input.Compilation.Resolutions...)
		current.Compilation.Resolutions[1].Agent = current.Compilation.Resolutions[0].Agent
		assert.False(t, ompModelDoctorProjectionMatches(receipt, current))
	})

	t.Run("receipt agent is not current", func(t *testing.T) {
		currentReceipt := receipt
		currentReceipt.Roles = append([]OMPModelRoleReceipt(nil), receipt.Roles...)
		currentReceipt.Roles[0].Agent = "unknown-agent"
		assert.False(t, ompModelDoctorProjectionMatches(currentReceipt, input))
	})
}

func TestReadOMPModelProjectOwnership_MissingMalformedAndDigestDrift(t *testing.T) {
	t.Parallel()

	ownership, exists, err := readOMPModelProjectOwnership(t.TempDir())
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, ownership.LedgerDigest)

	directoryRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(directoryRoot, OMPModelProjectOwnershipRelativePath), 0o700))
	_, exists, err = readOMPModelProjectOwnership(directoryRoot)
	assert.False(t, exists)
	assert.ErrorContains(t, err, "read project ownership ledger")

	malformedRoot := t.TempDir()
	writeOMPDoctorErrorFile(t, malformedRoot, OMPModelProjectOwnershipRelativePath, []byte("{}\n{}\n"), 0o600)
	_, exists, err = readOMPModelProjectOwnership(malformedRoot)
	assert.False(t, exists)
	assert.ErrorContains(t, err, "ledger invalid")

	driftRoot := t.TempDir()
	canonical, _, err := newOMPModelProjectOwnership(nil, true, []byte("emitted"), map[string]string{})
	require.NoError(t, err)
	canonical.LedgerDigest = doctorHash("d")
	data, err := json.Marshal(canonical)
	require.NoError(t, err)
	writeOMPDoctorErrorFile(t, driftRoot, OMPModelProjectOwnershipRelativePath, data, 0o600)
	_, exists, err = readOMPModelProjectOwnership(driftRoot)
	assert.False(t, exists)
	assert.ErrorContains(t, err, "ledger invalid")
}

func TestOMPModelProjectOriginHelpers_RequireExactLedgerEntry(t *testing.T) {
	t.Parallel()

	_, err := ompModelProjectOriginLedgerBytes(ompModelProjectOwnership{Salt: "bad"})
	assert.Error(t, err)

	journal := &adapter.TransactionJournal{Entries: []adapter.TransactionJournalEntry{
		{Path: OMPModelProjectOwnershipRelativePath, MissingBefore: true, AfterChecksum: "checksum"},
	}}
	assert.True(t, journalContainsOriginLedger(journal, "checksum"))
	assert.False(t, journalContainsOriginLedger(journal, "different"))
	assert.False(t, journalContainsOriginLedger(&adapter.TransactionJournal{}, "checksum"))
}

func writeOMPDoctorErrorReceipt(t *testing.T, root string) {
	t.Helper()
	_, err := WriteOMPModelResolutionReceipt(OMPModelReceiptWriteInput{
		WorkspaceRoot: root,
		Receipt:       modelReceiptFixture(time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)),
	})
	require.NoError(t, err)
}

func writeOMPDoctorErrorFile(t *testing.T, root, relative string, data []byte, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, data, mode))
}
