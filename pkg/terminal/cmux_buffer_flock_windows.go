//go:build windows

package terminal

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func tryLockCmuxBufferFile(file *os.File) (bool, error) {
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &windows.Overlapped{},
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock cmux input buffer: %w", err)
	}
	return false, nil
}

func unlockCmuxBufferFile(file *os.File) error {
	return windows.UnlockFileEx(
		windows.Handle(file.Fd()), 0, 1, 0, &windows.Overlapped{},
	)
}

func cmuxBufferLockOwnedByCurrentUser(os.FileInfo) bool { return true }
