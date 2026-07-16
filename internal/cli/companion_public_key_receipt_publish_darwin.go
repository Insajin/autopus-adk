//go:build darwin

package cli

import "golang.org/x/sys/unix"

func publishPublicKeyReceiptBundleNoReplace(
	parentFD int,
	stageName, bundleName string,
) error {
	return unix.RenameatxNp(
		parentFD,
		stageName,
		parentFD,
		bundleName,
		unix.RENAME_EXCL,
	)
}
