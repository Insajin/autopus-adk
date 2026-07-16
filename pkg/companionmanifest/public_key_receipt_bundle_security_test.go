//go:build darwin || linux

package companionmanifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func anchoredPublicKeyReceiptBundleFixture(
	t *testing.T,
) (string, publicKeyReceiptA0Anchor) {
	t.Helper()
	receiptBytes, signature, _, publicKey := issuedPublicKeyReceipt(t)
	return writePublicKeyReceiptTrustBundle(t, receiptBytes, signature),
		testPublicKeyReceiptA0Anchor(t, receiptBytes, signature, publicKey)
}

func TestVerifyPublicKeyReceiptBundle_PostVerificationSubstitution_FailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) error
	}{
		{
			name: "parent replacement",
			mutate: func(bundlePath string) error {
				parent := filepath.Dir(bundlePath)
				if err := os.Rename(parent, parent+".saved"); err != nil {
					return err
				}
				return os.Mkdir(parent, 0o700)
			},
		},
		{
			name: "bundle replacement",
			mutate: func(bundlePath string) error {
				if err := os.Rename(bundlePath, bundlePath+".saved"); err != nil {
					return err
				}
				return os.Mkdir(bundlePath, 0o700)
			},
		},
		{
			name: "entry swap",
			mutate: func(bundlePath string) error {
				entry := filepath.Join(bundlePath, publicKeyReceiptBundleEntryName)
				data, err := os.ReadFile(entry)
				if err != nil {
					return err
				}
				if err := os.Rename(entry, entry+".saved"); err != nil {
					return err
				}
				return os.WriteFile(entry, data, 0o600)
			},
		},
		{
			name: "extra entry",
			mutate: func(bundlePath string) error {
				return os.WriteFile(filepath.Join(bundlePath, "unexpected"), []byte("x"), 0o600)
			},
		},
		{
			name: "entry symlink",
			mutate: func(bundlePath string) error {
				entry := filepath.Join(bundlePath, publicKeyReceiptBundleEntryName)
				if err := os.Rename(entry, entry+".saved"); err != nil {
					return err
				}
				return os.Symlink(filepath.Base(entry)+".saved", entry)
			},
		},
	}
	accepted := 0
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundlePath, anchor := anchoredPublicKeyReceiptBundleFixture(t)
			attempted := false
			_, err := verifyPublicKeyReceiptBundle(
				bundlePath,
				validPublicKeyReceiptPolicy(),
				anchor,
				func(step string) error {
					if step != publicKeyReceiptBundleAfterVerification {
						return nil
					}
					if mutationErr := test.mutate(bundlePath); mutationErr != nil {
						return mutationErr
					}
					attempted = true
					return nil
				},
			)
			if err == nil {
				accepted++
			}
			if !attempted || err == nil {
				t.Fatalf("substitution attempted/error = %v/%v", attempted, err)
			}
		})
	}
	if accepted != 0 {
		t.Fatalf("accepted substituted bundles = %d, want 0", accepted)
	}
}

func TestVerifyPublicKeyReceiptBundle_MalformedAuthority_FailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) error
	}{
		{
			name: "bundle symlink",
			mutate: func(bundlePath string) error {
				if err := os.Rename(bundlePath, bundlePath+".saved"); err != nil {
					return err
				}
				return os.Symlink(filepath.Base(bundlePath)+".saved", bundlePath)
			},
		},
		{
			name: "nonregular entry",
			mutate: func(bundlePath string) error {
				entry := filepath.Join(bundlePath, publicKeyReceiptBundleSignatureEntryName)
				if err := os.Remove(entry); err != nil {
					return err
				}
				return os.Mkdir(entry, 0o700)
			},
		},
		{
			name: "duplicate inode",
			mutate: func(bundlePath string) error {
				signature := filepath.Join(bundlePath, publicKeyReceiptBundleSignatureEntryName)
				if err := os.Remove(signature); err != nil {
					return err
				}
				return os.Link(filepath.Join(bundlePath, publicKeyReceiptBundleEntryName), signature)
			},
		},
		{
			name: "extra entry",
			mutate: func(bundlePath string) error {
				return os.WriteFile(filepath.Join(bundlePath, "third"), []byte("x"), 0o600)
			},
		},
	}
	accepted := 0
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundlePath, anchor := anchoredPublicKeyReceiptBundleFixture(t)
			attempted := false
			if err := test.mutate(bundlePath); err != nil {
				t.Fatal(err)
			}
			attempted = true
			_, err := verifyPublicKeyReceiptBundle(
				bundlePath,
				validPublicKeyReceiptPolicy(),
				anchor,
				nil,
			)
			if err == nil {
				accepted++
			}
			if !attempted || err == nil || errors.Is(err, os.ErrNotExist) {
				t.Fatalf("substitution attempted/error = %v/%v", attempted, err)
			}
		})
	}
	if accepted != 0 {
		t.Fatalf("accepted malformed bundles = %d, want 0", accepted)
	}
}
