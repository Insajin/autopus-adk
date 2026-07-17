//go:build darwin

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
			return unix.RenamexNp(left, right, unix.RENAME_SWAP)
		},
		syncDirectory,
	)
}
