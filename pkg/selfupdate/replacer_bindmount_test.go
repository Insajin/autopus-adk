package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceWithOps_BindMountCommitWithoutStageIsSuccess(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	external := t.TempDir()
	backupPath := filepath.Join(external, "old-backup")
	var stagedInfo os.FileInfo
	ops := defaultReplaceOps()
	ops.commit = func(stagePath, gotTargetPath string, expected os.FileInfo) error {
		var err error
		stagedInfo, err = os.Lstat(stagePath)
		if err != nil {
			return err
		}
		calls := 0
		return commitWithAtomicSwap(stagePath, gotTargetPath, expected,
			func(left, right string) error {
				calls++
				if calls != 1 {
					return os.ErrNotExist
				}
				if err := os.Rename(right, backupPath); err != nil {
					return err
				}
				return os.Rename(left, right)
			}, func(string) error { return nil })
	}

	require.NoError(t, replaceWithOps(newBinaryPath, targetPath, ops))
	assertInstalledBinary(t, targetPath, "new", 0o711)
	targetInfo, err := os.Lstat(targetPath)
	require.NoError(t, err)
	require.True(t, os.SameFile(stagedInfo, targetInfo),
		"committed target must be the pre-swap staged binary")
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_BindMountMissingStageWithDifferentTargetFailsClosed(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	external := t.TempDir()
	backupPath := filepath.Join(external, "old-backup")
	consumedStagePath := filepath.Join(external, "consumed-stage")
	ops := defaultReplaceOps()
	ops.commit = func(stagePath, gotTargetPath string, expected os.FileInfo) error {
		calls := 0
		return commitWithAtomicSwap(stagePath, gotTargetPath, expected,
			func(left, right string) error {
				calls++
				if calls != 1 {
					return os.ErrNotExist
				}
				if err := os.Rename(right, backupPath); err != nil {
					return err
				}
				if err := os.Rename(left, consumedStagePath); err != nil {
					return err
				}
				return os.WriteFile(right, []byte("different"), 0o700)
			}, func(string) error { return nil })
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.Error(t, err)
	require.False(t, preservesReplacementStage(err))
	require.NotContains(t, err.Error(), "recovery file preserved at")
	require.ErrorIs(t, err, os.ErrNotExist)
	assertInstalledBinary(t, targetPath, "different", 0o700)
	assertNoReplacementResidue(t, targetPath)
}

func TestPreserveStageErrorRequiresExistingRecoveryFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing-auto")
	err := rollbackAtomicSwap(
		missing, filepath.Join(dir, "auto"), errors.New("stage vanished"),
		func(string, string) error { return os.ErrNotExist },
		func(string) error { return nil },
	)
	require.Error(t, err)
	require.False(t, preservesReplacementStage(err))
	require.NotContains(t, err.Error(), missing)
	require.NotContains(t, err.Error(), "recovery file preserved at")
}

func TestReplaceWithOps_ReportsCleanupFailure(t *testing.T) {
	t.Parallel()

	t.Run("after successful commit", func(t *testing.T) {
		newBinaryPath, targetPath := writeReplacementFixture(t)
		cleanupErr := errors.New("injected cleanup failure")
		ops := defaultReplaceOps()
		ops.commit = func(stagePath, targetPath string, _ os.FileInfo) error {
			if err := os.Remove(targetPath); err != nil {
				return err
			}
			return os.Rename(stagePath, targetPath)
		}
		ops.removeAll = func(string) error { return cleanupErr }

		err := replaceWithOps(newBinaryPath, targetPath, ops)
		require.ErrorIs(t, err, cleanupErr)
		require.Contains(t, err.Error(), "cleanup replacement directory")
		assertInstalledBinary(t, targetPath, "new", 0o711)
	})

	t.Run("alongside commit failure", func(t *testing.T) {
		newBinaryPath, targetPath := writeReplacementFixture(t)
		commitErr := errors.New("injected commit failure")
		cleanupErr := errors.New("injected cleanup failure")
		ops := defaultReplaceOps()
		ops.commit = func(string, string, os.FileInfo) error { return commitErr }
		ops.removeAll = func(string) error { return cleanupErr }

		err := replaceWithOps(newBinaryPath, targetPath, ops)
		require.ErrorIs(t, err, commitErr)
		require.ErrorIs(t, err, cleanupErr)
		assertInstalledBinary(t, targetPath, "old", 0o711)
	})
}
