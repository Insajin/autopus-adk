package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommitStagedBinaryWindows_RestoreFailurePreservesRecovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	installErr := errors.New("injected install failure")
	restoreErr := errors.New("injected restore failure")
	moveCalls := 0
	ops := windowsCommitOps{remove: os.Remove, lstat: os.Lstat}
	ops.move = func(sourcePath, destinationPath string) error {
		moveCalls++
		switch moveCalls {
		case 1:
			return os.Rename(sourcePath, destinationPath)
		case 2:
			return installErr
		default:
			return restoreErr
		}
	}

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, ops)
	require.ErrorIs(t, err, installErr)
	require.ErrorIs(t, err, restoreErr)
	require.ErrorContains(t, err, targetPath+".old")
	assertInstalledBinary(t, targetPath+".old", "old", 0711)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestCommitStagedBinaryWindows_RestoresConcurrentTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	require.NoError(t, os.Remove(targetPath))
	require.NoError(t, os.WriteFile(targetPath, []byte("concurrent"), 0700))
	stagePath := filepath.Join(dir, "new.exe")
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove: os.Remove,
		lstat:  os.Lstat,
		move:   os.Rename,
	})
	require.ErrorContains(t, err, "changed before Windows commit")
	assertInstalledBinary(t, targetPath, "concurrent", 0700)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestCommitStagedBinaryWindows_ExistingRecoveryFailsClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	oldPath := targetPath + ".old"
	require.NoError(t, os.WriteFile(targetPath, []byte("current"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	require.NoError(t, os.WriteFile(oldPath, []byte("recovery"), 0700))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	removeCalls := 0
	moveCalls := 0

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove: func(string) error {
			removeCalls++
			return nil
		},
		lstat: os.Lstat,
		move: func(string, string) error {
			moveCalls++
			return nil
		},
	})

	require.ErrorContains(t, err, "unresolved Windows recovery binary")
	require.ErrorContains(t, err, oldPath)
	require.Zero(t, removeCalls)
	require.Zero(t, moveCalls)
	assertInstalledBinary(t, targetPath, "current", 0711)
	assertInstalledBinary(t, stagePath, "new", 0711)
	assertInstalledBinary(t, oldPath, "recovery", 0700)
}

func TestCommitStagedBinaryWindows_CleanupFailureRecordsCompletedReplacement(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	sentinel := errors.New("injected cleanup failure")
	oldPath := targetPath + ".old"

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove: func(path string) error {
			if path == oldPath {
				return sentinel
			}
			return os.Remove(path)
		},
		lstat:       os.Lstat,
		move:        os.Rename,
		readFile:    os.ReadFile,
		writeMarker: writeWindowsRecoveryMarker,
	})

	require.NoError(t, err)
	assertInstalledBinary(t, targetPath, "new", 0711)
	assertInstalledBinary(t, oldPath, "old", 0711)
	marker, err := os.ReadFile(windowsRecoveryMarkerPath(oldPath))
	require.NoError(t, err)
	require.Equal(t, windowsRecoveryComplete, string(marker))
}

func TestCommitStagedBinaryWindows_CleansCompletedRecoveryBeforeNextCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	oldPath := targetPath + ".old"
	markerPath := windowsRecoveryMarkerPath(oldPath)
	require.NoError(t, os.WriteFile(targetPath, []byte("current"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	require.NoError(t, os.WriteFile(oldPath, []byte("completed-old"), 0700))
	require.NoError(t, os.WriteFile(markerPath, []byte(windowsRecoveryComplete), 0600))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove:      os.Remove,
		lstat:       os.Lstat,
		move:        os.Rename,
		readFile:    os.ReadFile,
		writeMarker: writeWindowsRecoveryMarker,
	})

	require.NoError(t, err)
	assertInstalledBinary(t, targetPath, "new", 0711)
	require.NoFileExists(t, oldPath)
	require.NoFileExists(t, markerPath)
}

func TestCommitStagedBinaryWindows_MarkerFailureKeepsRecoveryActionable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	oldPath := targetPath + ".old"
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	cleanupErr := errors.New("injected cleanup failure")
	markerErr := errors.New("injected marker failure")

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove: func(path string) error {
			if path == oldPath {
				return cleanupErr
			}
			return os.Remove(path)
		},
		lstat:    os.Lstat,
		move:     os.Rename,
		readFile: os.ReadFile,
		writeMarker: func(string) error {
			return markerErr
		},
	})

	require.ErrorIs(t, err, cleanupErr)
	require.ErrorIs(t, err, markerErr)
	require.ErrorContains(t, err, "new binary installed")
	require.ErrorContains(t, err, oldPath)
	assertInstalledBinary(t, targetPath, "new", 0711)
	assertInstalledBinary(t, oldPath, "old", 0711)
}

func TestCommitStagedBinaryWindows_RecoveryInspectionFailureStopsBeforeMove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	sentinel := errors.New("injected lstat failure")
	moveCalls := 0

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove: os.Remove,
		lstat:  func(string) (os.FileInfo, error) { return nil, sentinel },
		move: func(string, string) error {
			moveCalls++
			return nil
		},
	})

	require.ErrorIs(t, err, sentinel)
	require.Zero(t, moveCalls)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestCommitStagedBinaryWindows_MovedTargetInspectionFailureRestores(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "new.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	sentinel := errors.New("injected moved-target lstat failure")
	oldLstatCalls := 0
	markerPath := windowsRecoveryMarkerPath(targetPath + ".old")

	err = commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove: os.Remove,
		lstat: func(path string) (os.FileInfo, error) {
			if path == markerPath {
				return nil, os.ErrNotExist
			}
			oldLstatCalls++
			if oldLstatCalls == 1 {
				return nil, os.ErrNotExist
			}
			return nil, sentinel
		},
		move: os.Rename,
	})

	require.ErrorIs(t, err, sentinel)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertInstalledBinary(t, stagePath, "new", 0711)
}

func TestPrepareWindowsRecoveryPath_RemovesOrphanCompletionMarker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "auto.exe.old")
	markerPath := windowsRecoveryMarkerPath(oldPath)
	require.NoError(t, os.WriteFile(markerPath, []byte(windowsRecoveryComplete), 0600))

	require.NoError(t, prepareWindowsRecoveryPath(oldPath, markerPath, windowsCommitOps{
		remove: os.Remove,
		lstat:  os.Lstat,
	}))
	require.NoFileExists(t, markerPath)
}

func TestPrepareWindowsRecoveryPath_RejectsInvalidCompletionMarker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "auto.exe.old")
	markerPath := windowsRecoveryMarkerPath(oldPath)
	require.NoError(t, os.WriteFile(oldPath, []byte("recovery"), 0700))
	invalid := make([]byte, len(windowsRecoveryComplete))
	require.NoError(t, os.WriteFile(markerPath, invalid, 0600))

	err := prepareWindowsRecoveryPath(oldPath, markerPath, windowsCommitOps{
		remove:   os.Remove,
		lstat:    os.Lstat,
		readFile: os.ReadFile,
	})
	require.ErrorContains(t, err, "completion marker is invalid")
	assertInstalledBinary(t, oldPath, "recovery", 0700)
	require.FileExists(t, markerPath)
}
