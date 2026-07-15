//go:build !darwin && !linux

package cli

import (
	"crypto/ed25519"
	"errors"
)

func readPublicKeyReceiptSigningKey(
	_ string,
	_ func(string) error,
) (ed25519.PrivateKey, error) {
	return nil, errors.New("secure public key receipt key input is unsupported")
}

func writePublicKeyReceiptBundle(
	_ publicKeyReceiptBundleOutput,
	_, _ []byte,
	_ func(string) error,
) error {
	return errors.New("secure public key receipt publication is unsupported")
}
