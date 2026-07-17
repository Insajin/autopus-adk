//go:build linux

package selfupdate

import (
	"os"

	"golang.org/x/sys/unix"
)

func commitStagedBinary(stagePath, targetPath string, expected os.FileInfo) error {
	return commitWithAtomicSwap(
		stagePath,
		targetPath,
		expected,
		func(left, right string) error {
			return unix.Renameat2(
				unix.AT_FDCWD,
				left,
				unix.AT_FDCWD,
				right,
				unix.RENAME_EXCHANGE,
			)
		},
		syncDirectory,
	)
}
