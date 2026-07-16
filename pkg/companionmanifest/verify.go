package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"
)

// PinnedKey is release-policy trust material, including its independent expiry.
type PinnedKey struct {
	PublicKey ed25519.PublicKey
	ExpiresAt string
}

// VerificationPolicy contains all caller-owned target and rollback expectations.
type VerificationPolicy struct {
	PinnedKeys           map[string]PinnedKey
	RevokedKeys          map[string]struct{}
	Now                  time.Time
	MinimumRollbackFloor uint64
	ExpectedPlatform     string
	ExpectedArchitecture string
	ExpectedHandoff      string
	ExpectedDigest       string
}

// Verify checks signature, key policy, time, target, rollback, and artifact bytes.
func Verify(
	manifestBytes, signature, artifact []byte,
	policy VerificationPolicy,
) (Manifest, error) {
	manifest, err := ParseStrict(manifestBytes)
	if err != nil {
		return Manifest{}, err
	}
	pinned, ok := policy.PinnedKeys[manifest.KeyID]
	if !ok {
		return Manifest{}, errors.New("unknown companion signing key")
	}
	if _, revoked := policy.RevokedKeys[manifest.KeyID]; revoked {
		return Manifest{}, errors.New("revoked companion signing key")
	}
	if len(pinned.PublicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize {
		return Manifest{}, errors.New("invalid companion signature material")
	}
	keyExpiry, err := parseCanonicalTime(pinned.ExpiresAt)
	if err != nil || policy.Now.IsZero() || !policy.Now.Before(keyExpiry) {
		return Manifest{}, errors.New("expired companion signing key")
	}
	issuedAt, _ := parseCanonicalTime(manifest.IssuedAt)
	manifestExpiry, _ := parseCanonicalTime(manifest.ExpiresAt)
	if policy.Now.Before(issuedAt) || !policy.Now.Before(manifestExpiry) {
		return Manifest{}, errors.New("companion manifest outside validity window")
	}
	if !ed25519.Verify(pinned.PublicKey, manifestBytes, signature) {
		return Manifest{}, errors.New("invalid companion signature")
	}
	if err := verifyTarget(manifest, artifact, policy); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func verifyTarget(manifest Manifest, artifact []byte, policy VerificationPolicy) error {
	if policy.ExpectedPlatform == "" || manifest.Platform != policy.ExpectedPlatform {
		return errors.New("companion platform mismatch")
	}
	if policy.ExpectedArchitecture == "" || manifest.Architecture != policy.ExpectedArchitecture {
		return errors.New("companion architecture mismatch")
	}
	if policy.ExpectedHandoff == "" || manifest.Handoff != policy.ExpectedHandoff {
		return errors.New("companion handoff mismatch")
	}
	if !digestPattern.MatchString(policy.ExpectedDigest) ||
		subtle.ConstantTimeCompare([]byte(manifest.ArtifactDigest), []byte(policy.ExpectedDigest)) != 1 {
		return errors.New("companion expected digest mismatch")
	}
	actualSum := sha256.Sum256(artifact)
	actualDigest := "sha256:" + hex.EncodeToString(actualSum[:])
	if subtle.ConstantTimeCompare([]byte(manifest.ArtifactDigest), []byte(actualDigest)) != 1 {
		return errors.New("companion artifact digest mismatch")
	}
	if manifest.RollbackFloor < policy.MinimumRollbackFloor {
		return errors.New("companion rollback floor violation")
	}
	return nil
}
