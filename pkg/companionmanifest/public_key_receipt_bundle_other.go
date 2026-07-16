//go:build !darwin && !linux

package companionmanifest

import "errors"

func verifyPublicKeyReceiptBundleAuthority(
	_ string,
	_ PublicKeyReceiptPolicy,
	_ publicKeyReceiptA0Anchor,
	_ publicKeyReceiptBundleHook,
) (TrustedPublicKeyReceipt, error) {
	return TrustedPublicKeyReceipt{}, errors.New(
		"secure public key receipt bundle verification is unsupported",
	)
}
