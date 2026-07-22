//go:build !darwin && !linux && !windows

package terminal

import (
	"errors"
	"os"
)

func tryLockCmuxBufferFile(*os.File) (bool, error) {
	return false, errors.New("cmux input buffer locking is unsupported")
}

func unlockCmuxBufferFile(*os.File) error {
	return errors.New("cmux input buffer unlocking is unsupported")
}

func cmuxBufferLockOwnedByCurrentUser(os.FileInfo) bool { return false }
