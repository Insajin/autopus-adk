//go:build darwin || linux

package companionmanifest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyPublicKeyReceiptBundle_InvalidTransactionInputsFailClosed(t *testing.T) {
	bundlePath, anchor := anchoredPublicKeyReceiptBundleFixture(t)
	tests := []struct {
		name string
		path string
	}{
		{name: "empty path", path: ""},
		{name: "missing parent", path: filepath.Join(t.TempDir(), "missing", "bundle")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifyPublicKeyReceiptBundle(
				test.path,
				validPublicKeyReceiptPolicy(),
				anchor,
				nil,
			); err == nil {
				t.Fatal("invalid bundle authority was accepted")
			}
		})
	}
	if _, err := verifyPublicKeyReceiptBundle(
		bundlePath,
		validPublicKeyReceiptPolicy(),
		publicKeyReceiptA0Anchor{},
		nil,
	); err == nil {
		t.Fatal("empty A0 anchor was accepted")
	}
}

func TestVerifyPublicKeyReceiptBundle_HookFailureReturnsNoCapability(t *testing.T) {
	bundlePath, anchor := anchoredPublicKeyReceiptBundleFixture(t)
	trusted, err := verifyPublicKeyReceiptBundle(
		bundlePath,
		validPublicKeyReceiptPolicy(),
		anchor,
		func(string) error { return errors.New("injected verification boundary failure") },
	)
	if err == nil || trusted.valid() {
		t.Fatalf("hook failure trust/error = %#v/%v", trusted, err)
	}
}

func TestVerifyPublicKeyReceiptBundle_EntrySizeAndModeFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) error
	}{
		{
			name: "empty receipt",
			mutate: func(bundlePath string) error {
				return os.WriteFile(
					filepath.Join(bundlePath, publicKeyReceiptBundleEntryName),
					nil,
					0o600,
				)
			},
		},
		{
			name: "oversize signature",
			mutate: func(bundlePath string) error {
				return os.WriteFile(
					filepath.Join(bundlePath, publicKeyReceiptBundleSignatureEntryName),
					[]byte(strings.Repeat("x", maxPublicKeyReceiptBundleEntryBytes+1)),
					0o600,
				)
			},
		},
		{
			name:   "insecure bundle mode",
			mutate: func(bundlePath string) error { return os.Chmod(bundlePath, 0o755) },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundlePath, anchor := anchoredPublicKeyReceiptBundleFixture(t)
			if err := test.mutate(bundlePath); err != nil {
				t.Fatal(err)
			}
			if _, err := verifyPublicKeyReceiptBundle(
				bundlePath,
				validPublicKeyReceiptPolicy(),
				anchor,
				nil,
			); err == nil {
				t.Fatal("invalid bundle entry was accepted")
			}
		})
	}
}

func TestPublicKeyReceiptBundleNames_InvalidDescriptorFailsClosed(t *testing.T) {
	if _, err := publicKeyReceiptBundleNames(-1); err == nil {
		t.Fatal("invalid bundle descriptor was listed")
	}
}
