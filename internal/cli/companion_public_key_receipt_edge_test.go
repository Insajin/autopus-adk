//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func TestCompanionPublicKeyReceiptBundle_NoReplacePublishRace_PreservesOccupant(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	raced := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "publish_ready" {
			return nil
		}
		if err := os.WriteFile(bundlePath, []byte("race-occupant"), 0o600); err != nil {
			return err
		}
		raced = true
		return nil
	})
	got, err := os.ReadFile(bundlePath)
	if result.err == nil || !raced || err != nil || string(got) != "race-occupant" {
		t.Fatalf("publish race result/bytes/error = %v / %q / %v", result.err, got, err)
	}
}

func TestCompanionPublicKeyReceiptBundle_ParentReplacementAfterFsync_RollsBackOriginal(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	parent := filepath.Dir(bundlePath)
	originalParent := parent + ".original"
	replaced := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "parent_dir_synced" {
			return nil
		}
		if err := os.Rename(parent, originalParent); err != nil {
			return err
		}
		if err := os.Mkdir(parent, 0o700); err != nil {
			return err
		}
		replaced = true
		return nil
	})
	if result.err == nil || !replaced {
		t.Fatalf("parent replacement result/attempted = %v / %v", result.err, replaced)
	}
	assertReceiptBundleAbsent(t, bundlePath)
	assertReceiptBundleAbsent(t, filepath.Join(originalParent, filepath.Base(bundlePath)))
}

func TestCompanionPublicKeyReceiptBundle_PublishedRegularSubstitution_IsQuarantined(t *testing.T) {
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
		if err := os.WriteFile(bundlePath, []byte("attacker-file"), 0o600); err != nil {
			return err
		}
		substituted = true
		return nil
	})
	if result.err == nil || !substituted {
		t.Fatalf("regular substitution result/attempted = %v / %v", result.err, substituted)
	}
	assertReceiptBundleAbsent(t, bundlePath)
	assertExactReceiptBundle(t, originalPath)
}

func TestCompanionPublicKeyReceiptBundle_InvalidIssuedRecord_FailsBeforePublish(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	command := newCompanionPublicKeyReceiptCmd()
	command.SetArgs([]string{
		"--key-file", keyPath, "--bundle-output", bundlePath,
		"--key-id", "unsafe key id", "--issued-at", "2026-07-14T00:00:00Z",
		"--expires-at", "2026-07-21T00:00:00Z", "--handoff", "v1",
		"--minimum-rollback-floor", "5069",
	})
	if err := command.Execute(); err == nil {
		t.Fatal("invalid receipt claims were accepted")
	}
	assertReceiptBundleAbsent(t, bundlePath)
}

func TestWritePublicKeyReceiptBundle_InvalidExactPair_RollsBack(t *testing.T) {
	_, bundlePath, privateKey := newReceiptBundleFixture(t)
	output, err := resolvePublicKeyReceiptBundle(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	receiptBytes, _, err := companionmanifest.IssuePublicKeyReceipt(
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
	for _, pair := range []struct {
		receipt   []byte
		signature []byte
	}{
		{receipt: nil, signature: []byte("signature")},
		{receipt: receiptBytes, signature: nil},
		{receipt: receiptBytes, signature: make([]byte, 64)},
	} {
		err := writePublicKeyReceiptBundle(output, pair.receipt, pair.signature, nil)
		if err == nil {
			t.Fatal("invalid exact receipt/signature pair was published")
		}
		assertReceiptBundleAbsent(t, bundlePath)
	}
}

func TestReadPublicKeyReceiptSigningKey_InvalidRootPathFailsClosed(t *testing.T) {
	if _, err := readPublicKeyReceiptSigningKey(string(filepath.Separator), nil); err == nil {
		t.Fatal("root directory was accepted as a private key")
	}
	if _, err := readPublicKeyReceiptSigningKey(
		filepath.Join(t.TempDir(), "missing", "key"),
		nil,
	); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing key parent error = %v", err)
	}
}
