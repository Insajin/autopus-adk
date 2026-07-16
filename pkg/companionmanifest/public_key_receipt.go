package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const (
	PublicKeyReceiptSchemaVersion = "adk-companion-public-key-receipt.v1"
	publicKeyReceiptAlgorithm     = "ed25519"
	publicKeyReceiptEncoding      = "base64-raw-32"
	minimumReceiptRollbackFloor   = 5069
	maxPublicKeyReceiptBytes      = 16 * 1024
)

// PublicKeyReceipt is the exact public A0/A1 key handoff record.
// Field order defines the canonical JSON order.
type PublicKeyReceipt struct {
	SchemaVersion        string `json:"schema_version"`
	KeyID                string `json:"key_id"`
	Algorithm            string `json:"algorithm"`
	PublicKeyEncoding    string `json:"public_key_encoding"`
	PublicKeyBase64      string `json:"public_key_base64"`
	PublicKeySHA256      string `json:"public_key_sha256"`
	IssuedAt             string `json:"issued_at"`
	ExpiresAt            string `json:"expires_at"`
	Handoff              string `json:"handoff"`
	MinimumRollbackFloor uint64 `json:"minimum_rollback_floor"`
}

// PublicKeyReceiptClaims are caller-owned release claims; key metadata is derived.
type PublicKeyReceiptClaims struct {
	KeyID                string
	IssuedAt             string
	ExpiresAt            string
	Handoff              string
	MinimumRollbackFloor uint64
}

// CanonicalPublicKeyReceiptBytes validates and serializes an exact compact receipt.
func CanonicalPublicKeyReceiptBytes(receipt PublicKeyReceipt) ([]byte, error) {
	if _, err := validatePublicKeyReceipt(receipt); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return nil, errors.New("encode public key receipt")
	}
	return encoded, nil
}

// ParsePublicKeyReceiptStrict accepts only the exact canonical v1 wire bytes.
func ParsePublicKeyReceiptStrict(data []byte) (PublicKeyReceipt, error) {
	if len(data) == 0 || len(data) > maxPublicKeyReceiptBytes || !utf8.Valid(data) {
		return PublicKeyReceipt{}, errors.New("invalid public key receipt bytes")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt PublicKeyReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return PublicKeyReceipt{}, errors.New("invalid public key receipt JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return PublicKeyReceipt{}, errors.New("trailing public key receipt value")
	}
	canonical, err := CanonicalPublicKeyReceiptBytes(receipt)
	if err != nil {
		return PublicKeyReceipt{}, err
	}
	if !bytes.Equal(data, canonical) {
		return PublicKeyReceipt{}, errors.New("non-canonical public key receipt")
	}
	return receipt, nil
}

// IssuePublicKeyReceipt derives public metadata and signs the canonical receipt.
func IssuePublicKeyReceipt(
	claims PublicKeyReceiptClaims,
	privateKey ed25519.PrivateKey,
) ([]byte, []byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("invalid receipt signing key")
	}
	seed := privateKey.Seed()
	defer clear(seed)
	normalizedKey := ed25519.NewKeyFromSeed(seed)
	defer clear(normalizedKey)
	if subtle.ConstantTimeCompare(privateKey, normalizedKey) != 1 {
		return nil, nil, errors.New("invalid receipt signing key")
	}
	publicKey := append(ed25519.PublicKey(nil), normalizedKey[ed25519.SeedSize:]...)
	digest := sha256.Sum256(publicKey)
	receipt := PublicKeyReceipt{
		SchemaVersion:        PublicKeyReceiptSchemaVersion,
		KeyID:                claims.KeyID,
		Algorithm:            publicKeyReceiptAlgorithm,
		PublicKeyEncoding:    publicKeyReceiptEncoding,
		PublicKeyBase64:      base64.StdEncoding.EncodeToString(publicKey),
		PublicKeySHA256:      "sha256:" + hex.EncodeToString(digest[:]),
		IssuedAt:             claims.IssuedAt,
		ExpiresAt:            claims.ExpiresAt,
		Handoff:              claims.Handoff,
		MinimumRollbackFloor: claims.MinimumRollbackFloor,
	}
	canonical, err := CanonicalPublicKeyReceiptBytes(receipt)
	if err != nil {
		return nil, nil, err
	}
	return canonical, ed25519.Sign(normalizedKey, canonical), nil
}

func validatePublicKeyReceipt(receipt PublicKeyReceipt) (ed25519.PublicKey, error) {
	if receipt.SchemaVersion != PublicKeyReceiptSchemaVersion {
		return nil, errors.New("unsupported public key receipt schema")
	}
	if !slugPattern.MatchString(receipt.KeyID) {
		return nil, errors.New("invalid public key receipt key_id")
	}
	if receipt.Algorithm != publicKeyReceiptAlgorithm || receipt.PublicKeyEncoding != publicKeyReceiptEncoding {
		return nil, errors.New("invalid public key receipt key metadata")
	}
	publicKey, err := base64.StdEncoding.Strict().DecodeString(receipt.PublicKeyBase64)
	if err != nil || len(publicKey) != ed25519.PublicKeySize ||
		base64.StdEncoding.EncodeToString(publicKey) != receipt.PublicKeyBase64 {
		return nil, errors.New("invalid public key receipt key encoding")
	}
	digest := sha256.Sum256(publicKey)
	wantDigest := "sha256:" + hex.EncodeToString(digest[:])
	if !digestPattern.MatchString(receipt.PublicKeySHA256) ||
		subtle.ConstantTimeCompare([]byte(receipt.PublicKeySHA256), []byte(wantDigest)) != 1 {
		return nil, errors.New("invalid public key receipt digest")
	}
	issuedAt, issuedErr := parseCanonicalTime(receipt.IssuedAt)
	expiresAt, expiresErr := parseCanonicalTime(receipt.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || !expiresAt.After(issuedAt) {
		return nil, errors.New("invalid public key receipt validity window")
	}
	if receipt.Handoff != "v1" || receipt.MinimumRollbackFloor < minimumReceiptRollbackFloor {
		return nil, errors.New("invalid public key receipt release policy")
	}
	return ed25519.PublicKey(publicKey), nil
}
