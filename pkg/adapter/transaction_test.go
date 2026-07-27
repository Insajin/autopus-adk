package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionApply_WriteFailureRollsBackCreatedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "blocked"), []byte("not a dir"), 0644))

	_, err := ApplyTransaction(root, "codex", TransactionPlan{
		Writes: []TransactionWrite{
			{Path: "created.txt", Content: []byte("created"), Perm: 0644},
			{Path: filepath.Join("blocked", "child.txt"), Content: []byte("boom"), Perm: 0644},
		},
	})

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(root, "created.txt"))
	assert.FileExists(t, filepath.Join(root, "blocked"))
}

func TestTransactionApply_WriteFailureRestoresExistingFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "blocked"), []byte("not a dir"), 0644))

	_, err := ApplyTransaction(root, "codex", TransactionPlan{
		Writes: []TransactionWrite{
			{Path: "existing.txt", Content: []byte("new"), Perm: 0644},
			{Path: filepath.Join("blocked", "child.txt"), Content: []byte("boom"), Perm: 0644},
		},
	})

	require.Error(t, err)
	data, readErr := os.ReadFile(filepath.Join(root, "existing.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(data))
}

func TestRollbackLatestTransaction_RestoresCommittedTransaction(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	journal, err := ApplyTransaction(root, "codex", TransactionPlan{
		Writes: []TransactionWrite{
			{Path: "created.txt", Content: []byte("created"), Perm: 0644},
		},
	})
	require.NoError(t, err)
	require.Equal(t, TransactionStatusCommitted, journal.Status)
	assert.FileExists(t, filepath.Join(root, "created.txt"))

	require.NoError(t, RollbackLatestTransaction(root, "codex"))
	assert.NoFileExists(t, filepath.Join(root, "created.txt"))
}

func TestTransactionApply_RejectsSymlinkedManagedParentWithoutExternalMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	external := filepath.Join(outside, "metrics.md")
	require.NoError(t, os.WriteFile(external, []byte("outside-owned\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755))
	if err := os.Symlink(outside, filepath.Join(root, ".claude", "skills", "autopus")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := ApplyTransaction(root, "claude-code", TransactionPlan{
		Removes: []TransactionRemove{{
			Path: filepath.Join(".claude", "skills", "autopus", "metrics.md"),
		}},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "crosses symlink")
	data, readErr := os.ReadFile(external)
	require.NoError(t, readErr)
	assert.Equal(t, "outside-owned\n", string(data))
}
