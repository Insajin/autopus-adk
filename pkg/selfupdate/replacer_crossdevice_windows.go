//go:build windows

package selfupdate

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

func isCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV) ||
		errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
