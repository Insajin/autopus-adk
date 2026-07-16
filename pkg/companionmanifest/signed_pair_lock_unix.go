//go:build darwin || linux

package companionmanifest

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockSignedPairFile(file *os.File) (bool, error) {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return true, nil
		}
		return false, errors.New("lock signed pair transaction")
	}
	return false, nil
}

func unlockSignedPairFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
