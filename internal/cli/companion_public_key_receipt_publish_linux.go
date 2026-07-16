//go:build linux

package cli

import "golang.org/x/sys/unix"

func publishPublicKeyReceiptBundleNoReplace(
	parentFD int,
	stageName, bundleName string,
) error {
	return unix.Renameat2(
		parentFD,
		stageName,
		parentFD,
		bundleName,
		unix.RENAME_NOREPLACE,
	)
}
