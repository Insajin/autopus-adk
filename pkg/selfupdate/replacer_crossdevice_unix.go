//go:build !windows

package selfupdate

import (
	"errors"
	"syscall"
)

func isCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
