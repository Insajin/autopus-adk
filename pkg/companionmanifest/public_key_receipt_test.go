package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	receiptVectorSeedHex      = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"
	receiptVectorPublicHex    = "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"
	receiptVectorPublicBase64 = "11qYAYKxCrfVS/7TyWQHOg7hcvPapiMlrwIaaPcHURo="
	receiptVectorPublicDigest = "sha256:21fe31dfa154a261626bf854046fd2271b7bed4b6abe45aa58877ef47f9721b9"
)

// This RFC 8032 vector is local test material and is not release evidence.
func receiptVectorKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	seed, err := hex.DecodeString(receiptVectorSeedHex)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	if hex.EncodeToString(publicKey) != receiptVectorPublicHex {
		t.Fatalf("derived public key = %x", publicKey)
	}
	return publicKey, privateKey
}

func validPublicKeyReceiptClaims() PublicKeyReceiptClaims {
	return PublicKeyReceiptClaims{
		KeyID:                "rfc8032-vector-1",
		IssuedAt:             "2026-07-14T00:00:00Z",
		ExpiresAt:            "2026-07-21T00:00:00Z",
		Handoff:              "v1",
		MinimumRollbackFloor: 5069,
	}
}

func issuedPublicKeyReceipt(t *testing.T) ([]byte, []byte, PublicKeyReceipt, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey := receiptVectorKeyPair(t)
	receiptBytes, signature, err := IssuePublicKeyReceipt(validPublicKeyReceiptClaims(), privateKey)
	if err != nil {
		t.Fatalf("IssuePublicKeyReceipt() error = %v", err)
	}
	receipt, err := ParsePublicKeyReceiptStrict(receiptBytes)
	if err != nil {
		t.Fatalf("ParsePublicKeyReceiptStrict() error = %v", err)
	}
	return receiptBytes, signature, receipt, publicKey
}

func TestCanonicalPublicKeyReceiptBytes_ValidReceipt_MatchesExactOrderedCompactUTF8(t *testing.T) {
	publicKey, _ := receiptVectorKeyPair(t)
	sum := sha256.Sum256(publicKey)
	receipt := PublicKeyReceipt{
		SchemaVersion:        PublicKeyReceiptSchemaVersion,
		KeyID:                "rfc8032-vector-1",
		Algorithm:            "ed25519",
		PublicKeyEncoding:    "base64-raw-32",
		PublicKeyBase64:      base64.StdEncoding.EncodeToString(publicKey),
		PublicKeySHA256:      "sha256:" + hex.EncodeToString(sum[:]),
		IssuedAt:             "2026-07-14T00:00:00Z",
		ExpiresAt:            "2026-07-21T00:00:00Z",
		Handoff:              "v1",
		MinimumRollbackFloor: 5069,
	}
	want := `{"schema_version":"adk-companion-public-key-receipt.v1","key_id":"rfc8032-vector-1","algorithm":"ed25519","public_key_encoding":"base64-raw-32","public_key_base64":"11qYAYKxCrfVS/7TyWQHOg7hcvPapiMlrwIaaPcHURo=","public_key_sha256":"sha256:21fe31dfa154a261626bf854046fd2271b7bed4b6abe45aa58877ef47f9721b9","issued_at":"2026-07-14T00:00:00Z","expires_at":"2026-07-21T00:00:00Z","handoff":"v1","minimum_rollback_floor":5069}`

	got, err := CanonicalPublicKeyReceiptBytes(receipt)
	if err != nil {
		t.Fatalf("CanonicalPublicKeyReceiptBytes() error = %v", err)
	}
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("canonical receipt = %q, want %q", got, want)
	}
	if !utf8.Valid(got) || bytes.Contains(got, []byte{'\n'}) || bytes.HasPrefix(got, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatalf("canonical receipt is not compact UTF-8 without LF/BOM: %q", got)
	}
}

func TestParsePublicKeyReceiptStrict_WireDrift_IsRejected(t *testing.T) {
	canonical, _, _, _ := issuedPublicKeyReceipt(t)
	prefix := `{"schema_version":"adk-companion-public-key-receipt.v1","key_id":"rfc8032-vector-1"`
	reordered := bytes.Replace(canonical, []byte(prefix), []byte(`{"key_id":"rfc8032-vector-1","schema_version":"adk-companion-public-key-receipt.v1"`), 1)
	invalidUTF8 := append([]byte(nil), canonical...)
	invalidUTF8[bytes.Index(invalidUTF8, []byte("rfc8032"))] = 0xff
	cases := []struct {
		name string
		data []byte
	}{
		{name: "unknown field", data: bytes.Replace(canonical, []byte(`}`), []byte(`,"unknown":true}`), 1)},
		{name: "duplicate field", data: bytes.Replace(canonical, []byte(`,"algorithm"`), []byte(`,"key_id":"duplicate","algorithm"`), 1)},
		{name: "missing field", data: bytes.Replace(canonical, []byte(`,"handoff":"v1"`), nil, 1)},
		{name: "order drift", data: reordered},
		{name: "pretty whitespace", data: append([]byte(" "), canonical...)},
		{name: "trailing LF", data: append(append([]byte(nil), canonical...), '\n')},
		{name: "UTF-8 BOM", data: append([]byte{0xef, 0xbb, 0xbf}, canonical...)},
		{name: "invalid UTF-8", data: invalidUTF8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePublicKeyReceiptStrict(tc.data); err == nil {
				t.Fatalf("ParsePublicKeyReceiptStrict(%q) error = nil", tc.data)
			}
		})
	}
}

func TestIssuePublicKeyReceipt_RFC8032Vector_DerivesRawPublicKeyAndRawSignature(t *testing.T) {
	receiptBytes, signature, receipt, publicKey := issuedPublicKeyReceipt(t)
	decoded, err := base64.StdEncoding.Strict().DecodeString(receipt.PublicKeyBase64)
	if err != nil {
		t.Fatalf("public key base64 error = %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize || !bytes.Equal(decoded, publicKey) {
		t.Fatalf("decoded public key = %x, want raw 32-byte %x", decoded, publicKey)
	}
	if receipt.PublicKeyBase64 != receiptVectorPublicBase64 || !strings.HasSuffix(receipt.PublicKeyBase64, "=") {
		t.Fatalf("public_key_base64 = %q", receipt.PublicKeyBase64)
	}
	if receipt.PublicKeySHA256 != receiptVectorPublicDigest {
		t.Fatalf("public_key_sha256 = %q", receipt.PublicKeySHA256)
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(publicKey, receiptBytes, signature) {
		t.Fatalf("detached signature length/verification = %d/%v", len(signature), ed25519.Verify(publicKey, receiptBytes, signature))
	}
}

func TestCanonicalPublicKeyReceiptBytes_InvalidSemanticFields_AreRejected(t *testing.T) {
	_, _, valid, _ := issuedPublicKeyReceipt(t)
	cases := []struct {
		name   string
		mutate func(*PublicKeyReceipt)
	}{
		{name: "schema", mutate: func(r *PublicKeyReceipt) { r.SchemaVersion = "future.v2" }},
		{name: "empty key id", mutate: func(r *PublicKeyReceipt) { r.KeyID = "" }},
		{name: "unsafe key id", mutate: func(r *PublicKeyReceipt) { r.KeyID = "bad key" }},
		{name: "leading punctuation key id", mutate: func(r *PublicKeyReceipt) { r.KeyID = "-bad" }},
		{name: "oversize key id", mutate: func(r *PublicKeyReceipt) { r.KeyID = "k" + strings.Repeat("x", 256) }},
		{name: "algorithm", mutate: func(r *PublicKeyReceipt) { r.Algorithm = "Ed25519" }},
		{name: "encoding", mutate: func(r *PublicKeyReceipt) { r.PublicKeyEncoding = "base64" }},
		{name: "unpadded base64", mutate: func(r *PublicKeyReceipt) { r.PublicKeyBase64 = strings.TrimSuffix(r.PublicKeyBase64, "=") }},
		{name: "URL base64", mutate: func(r *PublicKeyReceipt) { r.PublicKeyBase64 = strings.ReplaceAll(r.PublicKeyBase64, "/", "_") }},
		{name: "short public key", mutate: func(r *PublicKeyReceipt) { r.PublicKeyBase64 = "AA==" }},
		{name: "uppercase digest", mutate: func(r *PublicKeyReceipt) { r.PublicKeySHA256 = strings.ToUpper(r.PublicKeySHA256) }},
		{name: "digest mismatch", mutate: func(r *PublicKeyReceipt) { r.PublicKeySHA256 = "sha256:" + strings.Repeat("0", 64) }},
		{name: "fractional issued time", mutate: func(r *PublicKeyReceipt) { r.IssuedAt = "2026-07-14T00:00:00.001Z" }},
		{name: "non-UTC expiry", mutate: func(r *PublicKeyReceipt) { r.ExpiresAt = "2026-07-21T09:00:00+09:00" }},
		{name: "non-increasing time", mutate: func(r *PublicKeyReceipt) { r.ExpiresAt = r.IssuedAt }},
		{name: "handoff", mutate: func(r *PublicKeyReceipt) { r.Handoff = "v2" }},
		{name: "rollback floor", mutate: func(r *PublicKeyReceipt) { r.MinimumRollbackFloor = 5068 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := valid
			tc.mutate(&candidate)
			if _, err := CanonicalPublicKeyReceiptBytes(candidate); err == nil {
				t.Fatal("CanonicalPublicKeyReceiptBytes() error = nil")
			}
		})
	}
}
