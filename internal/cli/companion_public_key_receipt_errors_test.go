//go:build darwin || linux

package cli

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func TestCompanionPublicKeyReceiptBundle_PublishedDirectorySubstitution_IsQuarantined(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	originalPath := bundlePath + ".original"
	substituted := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "bundle_published" {
			return nil
		}
		if err := os.Rename(bundlePath, originalPath); err != nil {
			return err
		}
		if err := os.Mkdir(bundlePath, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(
			filepath.Join(bundlePath, "attacker"),
			[]byte("attacker-bytes"),
			0o600,
		); err != nil {
			return err
		}
		substituted = true
		return nil
	})
	if result.err == nil || !substituted {
		t.Fatalf("substitution result/attempted = %v / %v", result.err, substituted)
	}
	assertReceiptBundleAbsent(t, bundlePath)
	assertExactReceiptBundle(t, originalPath)
}

func TestPublicKeyReceiptBundleHelpers_InvalidInputsFailClosed(t *testing.T) {
	if _, err := resolvePublicKeyReceiptBundle(""); err == nil {
		t.Fatal("empty bundle path was accepted")
	}
	missingParent := filepath.Join(t.TempDir(), "missing", "receipt.bundle")
	output, err := resolvePublicKeyReceiptBundle(missingParent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := beginPublicKeyReceiptBundleTransaction(output); err == nil {
		t.Fatal("missing bundle parent was accepted")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o777); err != nil {
		t.Fatal(err)
	}
	output, err = resolvePublicKeyReceiptBundle(filepath.Join(root, "receipt.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := beginPublicKeyReceiptBundleTransaction(output); err == nil {
		t.Fatal("writable bundle namespace was accepted")
	}
}

func TestPublicKeyReceiptBundleDescriptorHelpers_RejectMalformedState(t *testing.T) {
	root := t.TempDir()
	directoryFD, err := openPublicKeyReceiptDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unix.Close(directoryFD) }()
	if err := writePublicKeyReceiptBundleEntry(directoryFD, "empty", nil); err == nil {
		t.Fatal("empty bundle entry was accepted")
	}
	oversize := []byte(strings.Repeat("x", maxPublicKeyReceiptBundleEntryBytes+1))
	if err := writePublicKeyReceiptBundleEntry(directoryFD, "large", oversize); err == nil {
		t.Fatal("oversize bundle entry was accepted")
	}
	if _, err := readPublicKeyReceiptBundleEntry(directoryFD, "missing"); err == nil {
		t.Fatal("missing bundle entry was read")
	}
	if _, err := publicKeyReceiptBundleEntryNames(-1); err == nil {
		t.Fatal("invalid bundle descriptor was listed")
	}
	if err := verifyPublicKeyReceiptBundleDescriptor(directoryFD, []byte("r"), []byte("s")); err == nil {
		t.Fatal("empty bundle descriptor passed verification")
	}
	if err := writePublicKeyReceiptBundleEntry(directoryFD, "duplicate", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := writePublicKeyReceiptBundleEntry(directoryFD, "duplicate", []byte("two")); err == nil {
		t.Fatal("existing bundle entry was replaced")
	}
	readOnly, err := os.Open(filepath.Join(root, "duplicate"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writePublicKeyReceiptDescriptor(readOnly, []byte("write")); err == nil {
		t.Fatal("write through read-only descriptor succeeded")
	}
	_ = readOnly.Close()
	badMode := filepath.Join(root, "bad-mode")
	if err := os.WriteFile(badMode, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPublicKeyReceiptBundleEntry(directoryFD, "bad-mode"); err == nil {
		t.Fatal("insecure bundle entry mode was read")
	}
	largePath := filepath.Join(root, "oversize")
	if err := os.WriteFile(largePath, oversize, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPublicKeyReceiptBundleEntry(directoryFD, "oversize"); err == nil {
		t.Fatal("oversize bundle entry was read")
	}
}

func TestCompanionPublicKeyReceiptBundle_OutputOccupiedBeforePublish_FailsWithoutOverwrite(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	occupied := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "before_publish" {
			return nil
		}
		if err := os.WriteFile(bundlePath, []byte("protected"), 0o600); err != nil {
			return err
		}
		occupied = true
		return nil
	})
	got, err := os.ReadFile(bundlePath)
	if result.err == nil || !occupied || err != nil || string(got) != "protected" {
		t.Fatalf("occupied result/bytes/error = %v / %q / %v", result.err, got, err)
	}
	if errors.Is(result.err, os.ErrNotExist) {
		t.Fatalf("unexpected occupied error = %v", result.err)
	}
}

func TestPublicKeyReceiptBundleDescriptor_InvalidReceiptOrSignatureFailsReverify(t *testing.T) {
	_, _, privateKey := newReceiptBundleFixture(t)
	receiptBytes, signature, err := companionmanifest.IssuePublicKeyReceipt(
		companionmanifest.PublicKeyReceiptClaims{
			KeyID: "rfc8032-vector-1", IssuedAt: "2026-07-14T00:00:00Z",
			ExpiresAt: "2026-07-21T00:00:00Z", Handoff: "v1",
			MinimumRollbackFloor: 5069,
		},
		privateKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		receipt   []byte
		signature []byte
	}{
		{name: "malformed receipt", receipt: []byte("not-json"), signature: signature},
		{name: "wrong signature", receipt: receiptBytes, signature: make([]byte, ed25519.SignatureSize)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			directoryFD, err := openPublicKeyReceiptDirectory(root)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = unix.Close(directoryFD) }()
			if err := writePublicKeyReceiptBundleEntry(
				directoryFD,
				publicKeyReceiptEntryName,
				tc.receipt,
			); err != nil {
				t.Fatal(err)
			}
			if err := writePublicKeyReceiptBundleEntry(
				directoryFD,
				publicKeyReceiptSignatureEntryName,
				tc.signature,
			); err != nil {
				t.Fatal(err)
			}
			if err := verifyPublicKeyReceiptBundleDescriptor(
				directoryFD,
				tc.receipt,
				tc.signature,
			); err == nil {
				t.Fatal("invalid receipt bundle passed signature reverify")
			}
		})
	}
}

func TestCompanionPublicKeyReceiptKey_MalformedEncodingFailsClosed(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	for _, content := range [][]byte{
		[]byte("not-base64"),
		[]byte(strings.Repeat("A", maxEncodedPrivateKeyBytes+1)),
	} {
		if err := os.WriteFile(keyPath, content, 0o600); err != nil {
			t.Fatal(err)
		}
		result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, nil)
		if result.err == nil {
			t.Fatal("malformed private key encoding was accepted")
		}
		assertReceiptBundleAbsent(t, bundlePath)
	}
}

func TestCompanionPublicKeyReceiptBundle_UnexpectedEntryOnRollbackClearsPublicName(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	added := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "bundle_published" {
			return nil
		}
		if err := os.WriteFile(
			filepath.Join(bundlePath, "unexpected"),
			[]byte("bytes"),
			0o600,
		); err != nil {
			return err
		}
		added = true
		return errors.New("force rollback")
	})
	if result.err == nil || !added {
		t.Fatalf("unexpected-entry result/attempted = %v / %v", result.err, added)
	}
	assertReceiptBundleAbsent(t, bundlePath)
}
