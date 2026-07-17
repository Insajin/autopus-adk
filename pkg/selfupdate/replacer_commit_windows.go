//go:build windows

package selfupdate

import (
	"os"

	"golang.org/x/sys/windows"
)

func commitStagedBinary(
	stagePath, targetPath string,
	expected os.FileInfo,
) error {
	return commitStagedBinaryWindows(stagePath, targetPath, expected, windowsCommitOps{
		remove:      os.Remove,
		lstat:       os.Lstat,
		move:        moveBinaryNoReplace,
		readFile:    os.ReadFile,
		writeMarker: writeWindowsRecoveryMarker,
	})
}

func moveBinaryNoReplace(sourcePath, targetPath string) error {
	source, err := windows.UTF16PtrFromString(sourcePath)
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(source, target, windows.MOVEFILE_WRITE_THROUGH)
}
