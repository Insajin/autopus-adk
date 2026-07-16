package cli

import (
	"errors"
	"path/filepath"
)

const (
	publicKeyReceiptEntryName           = "public-key-receipt.json"
	publicKeyReceiptSignatureEntryName  = "public-key-receipt.sig"
	publicKeyReceiptStagePrefix         = ".public-key-receipt.stage-"
	maxPublicKeyReceiptBundleEntryBytes = 16 * 1024
)

type publicKeyReceiptBundleOutput struct {
	bundlePath string
	parentPath string
	bundleName string
}

func resolvePublicKeyReceiptBundle(path string) (publicKeyReceiptBundleOutput, error) {
	clean := filepath.Clean(path)
	absolute, err := filepath.Abs(clean)
	if err != nil || clean == "." || filepath.Base(absolute) == "." ||
		filepath.Base(absolute) == string(filepath.Separator) {
		return publicKeyReceiptBundleOutput{}, errors.New("invalid receipt bundle output")
	}
	return publicKeyReceiptBundleOutput{
		bundlePath: absolute,
		parentPath: filepath.Dir(absolute),
		bundleName: filepath.Base(absolute),
	}, nil
}

func runPublicKeyReceiptFault(hook func(string) error, step string) error {
	if hook == nil {
		return nil
	}
	if err := hook(step); err != nil {
		return errors.New("public key receipt transaction fault")
	}
	return nil
}
