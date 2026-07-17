package selfupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
