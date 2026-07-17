//go:build !darwin && !linux && !windows

package selfupdate

import (
	"errors"
	"os"
)

func commitStagedBinary(_, _ string, _ os.FileInfo) error {
	return errors.New("atomic binary exchange is unsupported on this platform")
}
