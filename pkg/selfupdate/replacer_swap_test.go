package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommitWithAtomicSwap_RollsBackChangedTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	stageDir := filepath.Join(dir, "stage")
	require.NoError(t, os.Mkdir(stageDir, 0700))
	stagePath := filepath.Join(stageDir, "auto")
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	swapCalls := 0

	err = commitWithAtomicSwap(stagePath, targetPath, expected, func(left, right string) error {
		swapCalls++
		if swapCalls == 1 {
			require.NoError(t, os.Remove(targetPath))
			require.NoError(t, os.WriteFile(targetPath, []byte("concurrent"), 0700))
		}
		return exchangeForTest(left, right)
	}, func(string) error { return nil })

	require.ErrorContains(t, err, "changed before atomic commit")
	require.Equal(t, 2, swapCalls)
	assertInstalledBinary(t, targetPath, "concurrent", 0700)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestCommitWithAtomicSwap_PreservesStageWhenRollbackFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	stagePath := filepath.Join(dir, "new")
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	sentinel := errors.New("rollback failed")
	swapCalls := 0

	err = commitWithAtomicSwap(stagePath, targetPath, expected, func(left, right string) error {
		swapCalls++
		if swapCalls == 2 {
			return sentinel
		}
		require.NoError(t, os.Remove(targetPath))
		require.NoError(t, os.WriteFile(targetPath, []byte("concurrent"), 0700))
		return exchangeForTest(left, right)
	}, func(string) error { return nil })

	require.ErrorIs(t, err, sentinel)
	require.True(t, preservesReplacementStage(err))
	require.ErrorContains(t, err, stagePath)
	require.FileExists(t, stagePath)
}

func TestCommitWithAtomicSwap_PreCommitSyncFailureDoesNotSwap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto")
	stagePath := filepath.Join(dir, "new")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	sentinel := errors.New("pre-commit sync failed")
	swapCalls := 0

	err = commitWithAtomicSwap(stagePath, targetPath, expected, func(string, string) error {
		swapCalls++
		return nil
	}, func(string) error { return sentinel })

	require.ErrorIs(t, err, sentinel)
	require.Zero(t, swapCalls)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestCommitWithAtomicSwap_PostCommitSyncFailureRollsBack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	stageDir := filepath.Join(dir, "stage")
	require.NoError(t, os.Mkdir(stageDir, 0700))
	stagePath := filepath.Join(stageDir, "auto")
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	sentinel := errors.New("post-commit sync failed")
	swapCalls := 0
	syncCalls := 0

	err = commitWithAtomicSwap(stagePath, targetPath, expected, func(left, right string) error {
		swapCalls++
		return exchangeForTest(left, right)
	}, func(string) error {
		syncCalls++
		if syncCalls == 2 {
			return sentinel
		}
		return nil
	})

	require.ErrorIs(t, err, sentinel)
	require.Equal(t, 2, swapCalls)
	require.Equal(t, 4, syncCalls)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestCommitWithAtomicSwap_SyncsDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	stageDir := filepath.Join(dir, "stage")
	require.NoError(t, os.Mkdir(stageDir, 0700))
	stagePath := filepath.Join(stageDir, "auto")
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	var synced []string

	require.NoError(t, commitWithAtomicSwap(
		stagePath,
		targetPath,
		expected,
		exchangeForTest,
		func(path string) error {
			synced = append(synced, path)
			return nil
		},
	))
	require.Equal(t, []string{stageDir, dir, stageDir}, synced)
	assertInstalledBinary(t, targetPath, "new", 0711)
	assertInstalledBinary(t, stagePath, "old", 0711)
}

func exchangeForTest(left, right string) error {
	tempPath := left + ".swap"
	if err := os.Rename(left, tempPath); err != nil {
		return err
	}
	if err := os.Rename(right, left); err != nil {
		_ = os.Rename(tempPath, left)
		return err
	}
	return os.Rename(tempPath, right)
}
