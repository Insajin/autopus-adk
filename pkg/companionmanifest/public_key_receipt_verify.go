package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"time"
)

// PublicKeyReceiptPolicy contains the consumer's expected public handoff claims.
type PublicKeyReceiptPolicy struct {
	Now                  time.Time
	ExpectedKeyID        string
	ExpectedHandoff      string
	MinimumRollbackFloor uint64
}

// CheckPublicKeyReceiptSelfConsistency checks only self-signed receipt integrity.
// It deliberately returns no key or trust capability.
func CheckPublicKeyReceiptSelfConsistency(
	receiptBytes, signature []byte,
	policy PublicKeyReceiptPolicy,
) error {
	_, _, err := checkPublicKeyReceiptSelfConsistency(receiptBytes, signature, policy)
	return err
}

func checkPublicKeyReceiptSelfConsistency(
	receiptBytes, signature []byte,
	policy PublicKeyReceiptPolicy,
) (PublicKeyReceipt, ed25519.PublicKey, error) {
	receipt, err := ParsePublicKeyReceiptStrict(receiptBytes)
	if err != nil {
		return PublicKeyReceipt{}, nil, err
	}
	publicKey, err := validatePublicKeyReceipt(receipt)
	if err != nil {
		return PublicKeyReceipt{}, nil, err
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(publicKey, receiptBytes, signature) {
		return PublicKeyReceipt{}, nil, errors.New("invalid public key receipt signature")
	}
	issuedAt, _ := parseCanonicalTime(receipt.IssuedAt)
	expiresAt, _ := parseCanonicalTime(receipt.ExpiresAt)
	if policy.Now.IsZero() || policy.Now.Before(issuedAt) || !policy.Now.Before(expiresAt) {
		return PublicKeyReceipt{}, nil, errors.New("public key receipt outside validity window")
	}
	if policy.ExpectedKeyID == "" || receipt.KeyID != policy.ExpectedKeyID {
		return PublicKeyReceipt{}, nil, errors.New("public key receipt key_id mismatch")
	}
	if policy.ExpectedHandoff == "" || receipt.Handoff != policy.ExpectedHandoff {
		return PublicKeyReceipt{}, nil, errors.New("public key receipt handoff mismatch")
	}
	if receipt.MinimumRollbackFloor < policy.MinimumRollbackFloor {
		return PublicKeyReceipt{}, nil, errors.New("public key receipt rollback floor violation")
	}
	return receipt, publicKey, nil
}

// ValidateManifestPublicKeyReceipt validates claims only; it establishes no trust.
// Use VerifyManifestWithTrustedPublicKeyReceipt for the signing-key conjunction.
func ValidateManifestPublicKeyReceipt(manifest Manifest, receipt PublicKeyReceipt) error {
	if err := validateManifest(manifest); err != nil {
		return err
	}
	if _, err := validatePublicKeyReceipt(receipt); err != nil {
		return err
	}
	if manifest.KeyID != receipt.KeyID || manifest.Handoff != receipt.Handoff {
		return errors.New("manifest and public key receipt identity mismatch")
	}
	if manifest.RollbackFloor < receipt.MinimumRollbackFloor {
		return errors.New("manifest below public key receipt rollback floor")
	}
	manifestIssued, _ := parseCanonicalTime(manifest.IssuedAt)
	manifestExpires, _ := parseCanonicalTime(manifest.ExpiresAt)
	receiptIssued, _ := parseCanonicalTime(receipt.IssuedAt)
	receiptExpires, _ := parseCanonicalTime(receipt.ExpiresAt)
	if manifestIssued.Before(receiptIssued) || manifestExpires.After(receiptExpires) {
		return errors.New("manifest outside public key receipt validity window")
	}
	return nil
}

// SamePublicKeyReceiptRecord requires exact A0/A1 receipt and signature bytes.
func SamePublicKeyReceiptRecord(
	receiptA, signatureA, receiptB, signatureB []byte,
) bool {
	return bytes.Equal(receiptA, receiptB) && bytes.Equal(signatureA, signatureB)
}
