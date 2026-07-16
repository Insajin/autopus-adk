package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func validPublicKeyReceiptPolicy() PublicKeyReceiptPolicy {
	return PublicKeyReceiptPolicy{
		Now:                  time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		ExpectedKeyID:        "rfc8032-vector-1",
		ExpectedHandoff:      "v1",
		MinimumRollbackFloor: 5069,
	}
}

func TestCheckPublicKeyReceiptSelfConsistency_ValidSelfSignedReceipt_ReturnsNoTrustMaterial(t *testing.T) {
	receiptBytes, signature, _, _ := issuedPublicKeyReceipt(t)

	if err := CheckPublicKeyReceiptSelfConsistency(
		receiptBytes,
		signature,
		validPublicKeyReceiptPolicy(),
	); err != nil {
		t.Fatalf("CheckPublicKeyReceiptSelfConsistency() error = %v", err)
	}
}

func TestCheckPublicKeyReceiptSelfConsistency_InvalidInput_FailsClosed(t *testing.T) {
	receiptBytes, signature, _, _ := issuedPublicKeyReceipt(t)
	tamperedSignature := append([]byte(nil), signature...)
	tamperedSignature[0] ^= 1
	cases := []struct {
		name      string
		receipt   []byte
		signature []byte
		mutate    func(*PublicKeyReceiptPolicy)
	}{
		{name: "short signature", signature: signature[:ed25519.SignatureSize-1]},
		{name: "long signature", signature: append(append([]byte(nil), signature...), 0)},
		{name: "tampered signature", signature: tamperedSignature},
		{name: "wrong 64-byte signature", signature: make([]byte, ed25519.SignatureSize)},
		{name: "receipt tamper", receipt: bytes.Replace(receiptBytes, []byte("rfc8032-vector-1"), []byte("rfc8032-vector-2"), 1)},
		{name: "malformed base64", receipt: bytes.Replace(receiptBytes, []byte(receiptVectorPublicBase64), []byte(strings.TrimSuffix(receiptVectorPublicBase64, "=")), 1)},
		{name: "malformed digest", receipt: bytes.Replace(receiptBytes, []byte(receiptVectorPublicDigest), []byte("sha256:"+strings.Repeat("A", 64)), 1)},
		{name: "unsafe key id", receipt: bytes.Replace(receiptBytes, []byte("rfc8032-vector-1"), []byte("unsafe key id value"), 1)},
		{name: "future receipt", mutate: func(p *PublicKeyReceiptPolicy) { p.Now = time.Date(2026, 7, 13, 23, 59, 59, 0, time.UTC) }},
		{name: "expired receipt", mutate: func(p *PublicKeyReceiptPolicy) { p.Now = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC) }},
		{name: "wrong key id", mutate: func(p *PublicKeyReceiptPolicy) { p.ExpectedKeyID = "different-key" }},
		{name: "wrong handoff", mutate: func(p *PublicKeyReceiptPolicy) { p.ExpectedHandoff = "v2" }},
		{name: "rollback below policy", mutate: func(p *PublicKeyReceiptPolicy) { p.MinimumRollbackFloor = 5070 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidateReceipt := tc.receipt
			if candidateReceipt == nil {
				candidateReceipt = receiptBytes
			}
			candidateSignature := tc.signature
			if candidateSignature == nil {
				candidateSignature = signature
			}
			policy := validPublicKeyReceiptPolicy()
			if tc.mutate != nil {
				tc.mutate(&policy)
			}
			if err := CheckPublicKeyReceiptSelfConsistency(
				candidateReceipt,
				candidateSignature,
				policy,
			); err == nil {
				t.Fatal("CheckPublicKeyReceiptSelfConsistency() error = nil")
			}
		})
	}
}

func TestIssuePublicKeyReceipt_PrivateMaterial_IsAbsentFromJSONAndErrors(t *testing.T) {
	_, privateKey := receiptVectorKeyPair(t)
	seed := privateKey.Seed()
	receiptBytes, _, err := IssuePublicKeyReceipt(validPublicKeyReceiptClaims(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	invalidClaims := validPublicKeyReceiptClaims()
	invalidClaims.KeyID = "invalid key id"
	_, _, issueErr := IssuePublicKeyReceipt(invalidClaims, privateKey)
	if issueErr == nil {
		t.Fatal("IssuePublicKeyReceipt() error = nil")
	}
	_, _, keyErr := IssuePublicKeyReceipt(validPublicKeyReceiptClaims(), append(privateKey, 0))
	if keyErr == nil {
		t.Fatal("IssuePublicKeyReceipt(invalid key) error = nil")
	}
	errorsText := issueErr.Error() + "\n" + keyErr.Error()
	privateEncodings := [][]byte{
		seed,
		privateKey,
		[]byte(hex.EncodeToString(seed)),
		[]byte(hex.EncodeToString(privateKey)),
		[]byte(base64.StdEncoding.EncodeToString(seed)),
		[]byte(base64.StdEncoding.EncodeToString(privateKey)),
	}
	for _, forbidden := range privateEncodings {
		if bytes.Contains(receiptBytes, forbidden) || strings.Contains(errorsText, string(forbidden)) {
			t.Fatalf("private signing material appeared in receipt JSON or error")
		}
	}
}

func signedManifestForReceipt(t *testing.T, privateKey ed25519.PrivateKey) ([]byte, []byte, Manifest) {
	t.Helper()
	manifest := testManifest()
	manifest.KeyID = "rfc8032-vector-1"
	manifest.Handoff = "v1"
	manifest.RollbackFloor = 5069
	manifest.IssuedAt = "2026-07-14T00:00:01Z"
	manifest.ExpiresAt = "2026-07-20T23:59:59Z"
	manifestBytes, signature, err := SignCanonical(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return manifestBytes, signature, manifest
}

func TestManifestPublicKeyReceiptConjunction_ValidPair_BindsKeyHandoffFloorAndWindow(t *testing.T) {
	_, _, receipt, publicKey := issuedPublicKeyReceipt(t)
	_, privateKey := receiptVectorKeyPair(t)
	manifestBytes, manifestSignature, wantManifest := signedManifestForReceipt(t, privateKey)
	policy := validPolicy(publicKey)
	policy.PinnedKeys = map[string]PinnedKey{
		receipt.KeyID: {PublicKey: publicKey, ExpiresAt: receipt.ExpiresAt},
	}
	policy.Now = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	policy.MinimumRollbackFloor = receipt.MinimumRollbackFloor
	manifest, err := Verify(manifestBytes, manifestSignature, testArtifact, policy)
	if err != nil {
		t.Fatalf("Verify(manifest) error = %v", err)
	}
	if err := ValidateManifestPublicKeyReceipt(manifest, receipt); err != nil {
		t.Fatalf("ValidateManifestPublicKeyReceipt() error = %v", err)
	}
	if manifest != wantManifest || manifest.KeyID != receipt.KeyID || manifest.Handoff != receipt.Handoff ||
		manifest.RollbackFloor < receipt.MinimumRollbackFloor {
		t.Fatalf("manifest/receipt binding = %#v / %#v", manifest, receipt)
	}
}

func TestValidateManifestPublicKeyReceipt_ConjunctionDrift_IsRejected(t *testing.T) {
	_, _, receipt, _ := issuedPublicKeyReceipt(t)
	_, privateKey := receiptVectorKeyPair(t)
	_, _, manifest := signedManifestForReceipt(t, privateKey)
	boundary := manifest
	boundary.IssuedAt = receipt.IssuedAt
	boundary.ExpiresAt = receipt.ExpiresAt
	if err := ValidateManifestPublicKeyReceipt(boundary, receipt); err != nil {
		t.Fatalf("inclusive receipt window boundary rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*Manifest, *PublicKeyReceipt)
	}{
		{name: "key id", mutate: func(m *Manifest, _ *PublicKeyReceipt) { m.KeyID = "different-key" }},
		{name: "handoff", mutate: func(m *Manifest, _ *PublicKeyReceipt) { m.Handoff = "v2" }},
		{name: "floor", mutate: func(m *Manifest, _ *PublicKeyReceipt) { m.RollbackFloor = receipt.MinimumRollbackFloor - 1 }},
		{name: "manifest issued before receipt", mutate: func(m *Manifest, _ *PublicKeyReceipt) { m.IssuedAt = "2026-07-13T23:59:59Z" }},
		{name: "receipt issued after manifest", mutate: func(_ *Manifest, r *PublicKeyReceipt) { r.IssuedAt = "2026-07-14T00:00:02Z" }},
		{name: "empty manifest window", mutate: func(m *Manifest, _ *PublicKeyReceipt) { m.ExpiresAt = m.IssuedAt }},
		{name: "manifest expires after receipt", mutate: func(m *Manifest, _ *PublicKeyReceipt) { m.ExpiresAt = "2026-07-21T00:00:01Z" }},
		{name: "receipt expires before manifest", mutate: func(_ *Manifest, r *PublicKeyReceipt) { r.ExpiresAt = "2026-07-20T23:59:58Z" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestCandidate := manifest
			receiptCandidate := receipt
			tc.mutate(&manifestCandidate, &receiptCandidate)
			if err := ValidateManifestPublicKeyReceipt(manifestCandidate, receiptCandidate); err == nil {
				t.Fatal("ValidateManifestPublicKeyReceipt() error = nil")
			}
		})
	}
}

func TestSamePublicKeyReceiptRecord_A0A1_RequiresReceiptAndSignatureByteEquality(t *testing.T) {
	receiptA, signatureA, _, _ := issuedPublicKeyReceipt(t)
	_, privateKey := receiptVectorKeyPair(t)
	changedClaims := validPublicKeyReceiptClaims()
	changedClaims.ExpiresAt = "2026-07-20T23:59:59Z"
	receiptB, signatureB, err := IssuePublicKeyReceipt(changedClaims, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if !SamePublicKeyReceiptRecord(receiptA, signatureA, append([]byte(nil), receiptA...), append([]byte(nil), signatureA...)) {
		t.Fatal("byte-identical A0/A1 record was rejected")
	}
	if SamePublicKeyReceiptRecord(receiptA, signatureA, receiptB, signatureB) {
		t.Fatal("changed receipt counted as the same A0/A1 record")
	}
	changedSignature := append([]byte(nil), signatureA...)
	changedSignature[0] ^= 1
	if SamePublicKeyReceiptRecord(receiptA, signatureA, receiptA, changedSignature) {
		t.Fatal("changed detached signature counted as the same A0/A1 record")
	}
	if bytes.Equal(receiptA, receiptB) || bytes.Equal(signatureA, signatureB) {
		t.Fatal("changed receipt record unexpectedly remained byte-identical")
	}
}
