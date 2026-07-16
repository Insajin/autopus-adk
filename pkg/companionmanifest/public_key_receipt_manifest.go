package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
)

// VerifyManifestWithTrustedPublicKeyReceipt pins the exact manifest bytes,
// verifies them with the opaque A0-anchored key, and enforces release policy.
func VerifyManifestWithTrustedPublicKeyReceipt(
	manifestBytes, signature, artifact []byte,
	expectedManifestSHA256 string,
	policy VerificationPolicy,
	trusted TrustedPublicKeyReceipt,
) (Manifest, error) {
	if !trusted.valid() {
		return Manifest{}, errors.New("missing trusted public key receipt")
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	actualManifestSHA256 := "sha256:" + hex.EncodeToString(manifestDigest[:])
	if !digestPattern.MatchString(expectedManifestSHA256) || subtle.ConstantTimeCompare(
		[]byte(expectedManifestSHA256),
		[]byte(actualManifestSHA256),
	) != 1 {
		return Manifest{}, errors.New("companion manifest byte digest mismatch")
	}
	trustedKey := append(ed25519.PublicKey(nil), trusted.publicKey[:]...)
	trustedPolicy := policy
	trustedPolicy.PinnedKeys = map[string]PinnedKey{
		trusted.receipt.KeyID: {
			PublicKey: trustedKey,
			ExpiresAt: trusted.receipt.ExpiresAt,
		},
	}
	manifest, err := Verify(manifestBytes, signature, artifact, trustedPolicy)
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifestPublicKeyReceipt(manifest, trusted.receipt); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
