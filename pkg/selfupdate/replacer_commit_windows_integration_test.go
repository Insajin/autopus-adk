//go:build windows

package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCommitStagedBinaryWindows_Integration(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "auto-new.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))
	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)

	require.NoError(t, commitStagedBinary(stagePath, targetPath, expected))
	assertInstalledBinary(t, targetPath, "new", 0711)
	require.NoFileExists(t, stagePath)
	require.NoFileExists(t, targetPath+".old")
}

func TestCommitStagedBinaryWindows_RunningExecutableDefersCleanup(t *testing.T) {
	dir := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	targetPath := filepath.Join(dir, "auto.exe")
	stagePath := filepath.Join(dir, "auto-new.exe")
	readyPath := filepath.Join(dir, "helper.ready")
	stopPath := filepath.Join(dir, "helper.stop")
	require.NoError(t, copyFile(executable, targetPath))
	require.NoError(t, os.WriteFile(stagePath, []byte("new"), 0711))

	helper := exec.Command(targetPath, "-test.run=^TestWindowsSelfUpdateRunningExecutableHelper$")
	helper.Env = append(os.Environ(),
		"AUTOPUS_SELFUPDATE_HELPER=1",
		"AUTOPUS_SELFUPDATE_READY="+readyPath,
		"AUTOPUS_SELFUPDATE_STOP="+stopPath,
	)
	require.NoError(t, helper.Start())
	waited := false
	t.Cleanup(func() {
		_ = os.WriteFile(stopPath, []byte("stop"), 0600)
		if !waited && helper.Process != nil {
			_ = helper.Process.Kill()
			_ = helper.Wait()
		}
	})
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(readyPath)
		return statErr == nil
	}, 10*time.Second, 20*time.Millisecond)

	expected, err := os.Lstat(targetPath)
	require.NoError(t, err)
	require.NoError(t, commitStagedBinary(stagePath, targetPath, expected))
	assertInstalledContent(t, targetPath, "new")
	oldPath := targetPath + ".old"
	require.FileExists(t, oldPath)
	marker, err := os.ReadFile(windowsRecoveryMarkerPath(oldPath))
	require.NoError(t, err)
	require.Equal(t, windowsRecoveryComplete, string(marker))

	require.NoError(t, os.WriteFile(stopPath, []byte("stop"), 0600))
	require.NoError(t, helper.Wait())
	waited = true
	require.NoError(t, prepareWindowsRecoveryPath(
		oldPath,
		windowsRecoveryMarkerPath(oldPath),
		windowsCommitOps{
			remove:   os.Remove,
			lstat:    os.Lstat,
			readFile: os.ReadFile,
		},
	))
	require.NoFileExists(t, oldPath)
	require.NoFileExists(t, windowsRecoveryMarkerPath(oldPath))
}

func TestWindowsSelfUpdateRunningExecutableHelper(t *testing.T) {
	if os.Getenv("AUTOPUS_SELFUPDATE_HELPER") != "1" {
		t.Skip("helper subprocess only")
	}
	require.NoError(t, os.WriteFile(os.Getenv("AUTOPUS_SELFUPDATE_READY"), []byte("ready"), 0600))
	require.Eventually(t, func() bool {
		_, err := os.Stat(os.Getenv("AUTOPUS_SELFUPDATE_STOP"))
		return err == nil
	}, 30*time.Second, 20*time.Millisecond)
}
