package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPublicKeyReceiptA0Anchor(
	t *testing.T,
	receiptBytes, signature []byte,
	publicKey ed25519.PublicKey,
) publicKeyReceiptA0Anchor {
	t.Helper()
	receiptDigest := sha256.Sum256(receiptBytes)
	signatureDigest := sha256.Sum256(signature)
	publicKeyDigest := sha256.Sum256(publicKey)
	anchor, err := newPublicKeyReceiptA0Anchor(
		receiptDigest,
		signatureDigest,
		publicKeyDigest,
		publicKeyReceiptRecordDigest(receiptBytes, signature),
	)
	if err != nil {
		t.Fatal(err)
	}
	return anchor
}

func trustedPublicKeyReceiptFixture(t *testing.T) TrustedPublicKeyReceipt {
	t.Helper()
	receiptBytes, signature, _, publicKey := issuedPublicKeyReceipt(t)
	bundlePath := writePublicKeyReceiptTrustBundle(t, receiptBytes, signature)
	trusted, err := verifyPublicKeyReceiptBundle(
		bundlePath,
		validPublicKeyReceiptPolicy(),
		testPublicKeyReceiptA0Anchor(t, receiptBytes, signature, publicKey),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	return trusted
}

func writePublicKeyReceiptTrustBundle(
	t *testing.T,
	receiptBytes, signature []byte,
) string {
	t.Helper()
	bundlePath := filepath.Join(t.TempDir(), "receipt.bundle")
	if err := os.Mkdir(bundlePath, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		publicKeyReceiptBundleEntryName:          receiptBytes,
		publicKeyReceiptBundleSignatureEntryName: signature,
	} {
		if err := os.WriteFile(filepath.Join(bundlePath, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return bundlePath
}

func manifestSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func TestCheckPublicKeyReceiptSelfConsistency_AttackerKey_DoesNotEstablishA0Trust(t *testing.T) {
	receiptBytes, signature, _, publicKey := issuedPublicKeyReceipt(t)
	anchor := testPublicKeyReceiptA0Anchor(t, receiptBytes, signature, publicKey)
	attackerSeed := make([]byte, ed25519.SeedSize)
	for index := range attackerSeed {
		attackerSeed[index] = byte(index + 71)
	}
	attackerKey := ed25519.NewKeyFromSeed(attackerSeed)
	attackerReceipt, attackerSignature, err := IssuePublicKeyReceipt(
		validPublicKeyReceiptClaims(),
		attackerKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPublicKeyReceiptSelfConsistency(
		attackerReceipt,
		attackerSignature,
		validPublicKeyReceiptPolicy(),
	); err != nil {
		t.Fatalf("attacker self-consistency check failed unexpectedly: %v", err)
	}
	attackerBundle := writePublicKeyReceiptTrustBundle(t, attackerReceipt, attackerSignature)
	if _, err := verifyPublicKeyReceiptBundle(
		attackerBundle,
		validPublicKeyReceiptPolicy(),
		anchor,
		nil,
	); err == nil {
		t.Fatal("same claims signed by attacker key established A0 trust")
	}
	trustedBundle := writePublicKeyReceiptTrustBundle(t, receiptBytes, signature)
	if _, err := verifyPublicKeyReceiptBundle(
		trustedBundle,
		validPublicKeyReceiptPolicy(),
		anchor,
		nil,
	); err != nil {
		t.Fatalf("anchored exact receipt was rejected: %v", err)
	}
}

func TestNewPublicKeyReceiptA0Anchor_EmptyEvidence_FailsClosed(t *testing.T) {
	var empty [sha256.Size]byte
	if _, err := newPublicKeyReceiptA0Anchor(empty, empty, empty, empty); err == nil {
		t.Fatal("empty A0 anchor was accepted")
	}
	if trusted := (TrustedPublicKeyReceipt{}); trusted.valid() {
		t.Fatal("zero opaque trusted receipt was valid")
	}
}

func TestVerifyConfiguredPublicKeyReceiptBundle_NonProductionRecord_FailsPinned(t *testing.T) {
	receiptBytes, signature, _, _ := issuedPublicKeyReceipt(t)
	bundlePath := writePublicKeyReceiptTrustBundle(t, receiptBytes, signature)

	_, err := VerifyConfiguredPublicKeyReceiptBundle(
		bundlePath,
		validPublicKeyReceiptPolicy(),
	)
	if err == nil || errors.Is(err, ErrPublicKeyReceiptA0Unprovisioned) {
		t.Fatalf("non-production configured bundle error = %v", err)
	}
}

func TestVerifyManifestWithTrustedPublicKeyReceipt_SameKeyIDDifferentKey_IsRejected(t *testing.T) {
	trusted := trustedPublicKeyReceiptFixture(t)
	attackerSeed := make([]byte, ed25519.SeedSize)
	for index := range attackerSeed {
		attackerSeed[index] = byte(255 - index)
	}
	attackerPrivate := ed25519.NewKeyFromSeed(attackerSeed)
	attackerPublic := attackerPrivate.Public().(ed25519.PublicKey)
	manifestBytes, manifestSignature, _ := signedManifestForReceipt(t, attackerPrivate)
	policy := validPolicy(attackerPublic)
	policy.PinnedKeys = map[string]PinnedKey{
		"rfc8032-vector-1": {
			PublicKey: attackerPublic,
			ExpiresAt: "2026-07-21T00:00:00Z",
		},
	}
	policy.Now = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	policy.MinimumRollbackFloor = 5069
	if _, err := Verify(manifestBytes, manifestSignature, testArtifact, policy); err != nil {
		t.Fatalf("precondition: attacker-only policy did not verify its manifest: %v", err)
	}
	if _, err := VerifyManifestWithTrustedPublicKeyReceipt(
		manifestBytes,
		manifestSignature,
		testArtifact,
		manifestSHA256(manifestBytes),
		policy,
		trusted,
	); err == nil {
		t.Fatal("same key_id with a different actual signing key passed conjunction")
	}
}

func TestVerifyManifestWithTrustedPublicKeyReceipt_ExactSigningKey_Passes(t *testing.T) {
	trusted := trustedPublicKeyReceiptFixture(t)
	publicKey, privateKey := receiptVectorKeyPair(t)
	manifestBytes, manifestSignature, want := signedManifestForReceipt(t, privateKey)
	policy := validPolicy(publicKey)
	policy.PinnedKeys = nil
	policy.Now = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	policy.MinimumRollbackFloor = 5069
	got, err := VerifyManifestWithTrustedPublicKeyReceipt(
		manifestBytes,
		manifestSignature,
		testArtifact,
		manifestSHA256(manifestBytes),
		policy,
		trusted,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("manifest = %#v, want %#v", got, want)
	}
}

func TestVerifyManifestWithTrustedPublicKeyReceipt_ManifestDigestMismatch_IsRejected(t *testing.T) {
	trusted := trustedPublicKeyReceiptFixture(t)
	publicKey, privateKey := receiptVectorKeyPair(t)
	manifestBytes, manifestSignature, _ := signedManifestForReceipt(t, privateKey)
	policy := validPolicy(publicKey)
	policy.PinnedKeys = nil
	policy.Now = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	policy.MinimumRollbackFloor = 5069

	for name, expected := range map[string]string{
		"artifact digest is not manifest digest": policy.ExpectedDigest,
		"wrong manifest digest":                  "sha256:" + strings.Repeat("0", 64),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyManifestWithTrustedPublicKeyReceipt(
				manifestBytes,
				manifestSignature,
				testArtifact,
				expected,
				policy,
				trusted,
			); err == nil {
				t.Fatal("caller-owned manifest byte digest mismatch was accepted")
			}
		})
	}
}
