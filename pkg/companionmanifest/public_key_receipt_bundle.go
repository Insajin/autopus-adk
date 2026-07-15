package companionmanifest

const (
	publicKeyReceiptBundleEntryName          = "public-key-receipt.json"
	publicKeyReceiptBundleSignatureEntryName = "public-key-receipt.sig"
	publicKeyReceiptBundleAfterVerification  = "after_verification"
	maxPublicKeyReceiptBundleEntryBytes      = 16 * 1024
)

type publicKeyReceiptBundleHook func(string) error
type publicKeyReceiptA0AnchorLoader func() (publicKeyReceiptA0Anchor, error)

// VerifyConfiguredPublicKeyReceiptBundle verifies one directory authority
// against immutable release pins and returns an opaque trust capability.
func VerifyConfiguredPublicKeyReceiptBundle(
	bundlePath string,
	policy PublicKeyReceiptPolicy,
) (TrustedPublicKeyReceipt, error) {
	return verifyConfiguredPublicKeyReceiptBundle(
		bundlePath,
		policy,
		configuredPublicKeyReceiptA0Anchor,
	)
}

func verifyConfiguredPublicKeyReceiptBundle(
	bundlePath string,
	policy PublicKeyReceiptPolicy,
	loadAnchor publicKeyReceiptA0AnchorLoader,
) (TrustedPublicKeyReceipt, error) {
	anchor, err := loadAnchor()
	if err != nil {
		return TrustedPublicKeyReceipt{}, err
	}
	return verifyPublicKeyReceiptBundle(bundlePath, policy, anchor, nil)
}

func verifyPublicKeyReceiptBundle(
	bundlePath string,
	policy PublicKeyReceiptPolicy,
	anchor publicKeyReceiptA0Anchor,
	hook publicKeyReceiptBundleHook,
) (TrustedPublicKeyReceipt, error) {
	return verifyPublicKeyReceiptBundleAuthority(bundlePath, policy, anchor, hook)
}
