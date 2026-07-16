//go:build !darwin && !linux && !windows

package companionmanifest

import (
	"errors"
	"os"
)

func lockSignedPairFile(_ *os.File) (bool, error) {
	return false, errors.New("signed pair transaction locking is unsupported")
}

func unlockSignedPairFile(_ *os.File) error {
	return errors.New("signed pair transaction unlocking is unsupported")
}
