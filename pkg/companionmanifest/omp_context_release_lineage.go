package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

const (
	OMPContextReleaseLineageSchemaV1 = "autopus.omp_context_release_lineage.v1"
	ompContextReleaseLineageDomainV1 = "autopus.omp-context.release-lineage.v1\x00"
	ompContextReleaseLineageMaxBytes = 16 * 1024
)

var (
	ompContextReleaseHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	ompContextReleaseGitPattern  = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// OMPContextReleaseLineageV1 is the release-key-signed U-to-D mapping. U is
// the unsigned candidate artifact digest, not executable authority by itself;
// D is the measured installed executable digest.
type OMPContextReleaseLineageV1 struct {
	SchemaVersion    string `json:"schema_version"`
	KeyID            string `json:"key_id"`
	Algorithm        string `json:"algorithm"`
	UpstreamSHA256   string `json:"upstream_sha256"`
	ExecutableSHA256 string `json:"executable_sha256"`
	SourceRepository string `json:"source_repository"`
	SourceCommit     string `json:"source_commit"`
	SourceTree       string `json:"source_tree"`
	Target           string `json:"target"`
	Version          string `json:"version"`
}

// OMPContextReleaseLineagePolicy contains caller-owned release coordinates.
type OMPContextReleaseLineagePolicy struct {
	Now                      time.Time
	ExpectedKeyID            string
	ExpectedHandoff          string
	MinimumRollbackFloor     uint64
	ExpectedUpstreamSHA256   string
	ExpectedExecutableSHA256 string
	ExpectedSourceRepository string
	ExpectedSourceCommit     string
	ExpectedSourceTree       string
	ExpectedTarget           string
	ExpectedVersion          string
}

// VerifiedOMPContextReleaseLineage is an opaque release-key capability.
type VerifiedOMPContextReleaseLineage struct {
	lineage          OMPContextReleaseLineageV1
	capabilityDigest [sha256.Size]byte
	expiresAt        time.Time
}

// CanonicalOMPContextReleaseLineageBytes validates and serializes one lineage.
func CanonicalOMPContextReleaseLineageBytes(lineage OMPContextReleaseLineageV1) ([]byte, error) {
	if !validOMPContextReleaseLineage(lineage) {
		return nil, errors.New("invalid OMP context release lineage")
	}
	encoded, err := json.Marshal(lineage)
	if err != nil {
		return nil, errors.New("encode OMP context release lineage")
	}
	return encoded, nil
}

// OMPContextReleaseLineageSigningMessage returns the exact domain-separated
// bytes that the release key signs.
func OMPContextReleaseLineageSigningMessage(lineageBytes []byte) ([]byte, error) {
	if _, err := parseOMPContextReleaseLineage(lineageBytes); err != nil {
		return nil, err
	}
	message := make([]byte, 0, len(ompContextReleaseLineageDomainV1)+len(lineageBytes))
	message = append(message, ompContextReleaseLineageDomainV1...)
	message = append(message, lineageBytes...)
	return message, nil
}

// VerifyOMPContextReleaseLineage verifies an exact signed U-to-D mapping with
// an A0-anchored release-key capability.
func VerifyOMPContextReleaseLineage(lineageBytes, signature []byte,
	policy OMPContextReleaseLineagePolicy, trusted TrustedPublicKeyReceipt,
) (VerifiedOMPContextReleaseLineage, error) {
	if !trusted.valid() {
		return VerifiedOMPContextReleaseLineage{}, errors.New("missing trusted OMP context release key")
	}
	lineage, err := parseOMPContextReleaseLineage(lineageBytes)
	if err != nil {
		return VerifiedOMPContextReleaseLineage{}, err
	}
	if !validOMPContextReleaseLineagePolicy(policy) || !matchesOMPContextReleaseLineage(lineage, policy) {
		return VerifiedOMPContextReleaseLineage{}, errors.New("OMP context release lineage coordinate mismatch")
	}
	receipt, ok := trusted.Receipt()
	issuedAt, issuedErr := parseCanonicalTime(receipt.IssuedAt)
	expiresAt, expiresErr := parseCanonicalTime(receipt.ExpiresAt)
	if !ok || issuedErr != nil || expiresErr != nil || policy.Now.Before(issuedAt) || !policy.Now.Before(expiresAt) ||
		lineage.KeyID != receipt.KeyID || receipt.Handoff != policy.ExpectedHandoff ||
		receipt.MinimumRollbackFloor < policy.MinimumRollbackFloor {
		return VerifiedOMPContextReleaseLineage{}, errors.New("OMP context release key is outside authority")
	}
	message, err := OMPContextReleaseLineageSigningMessage(lineageBytes)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(ed25519.PublicKey(trusted.publicKey[:]), message, signature) {
		return VerifiedOMPContextReleaseLineage{}, errors.New("invalid OMP context release lineage signature")
	}
	verified := VerifiedOMPContextReleaseLineage{lineage: lineage, expiresAt: expiresAt}
	verified.capabilityDigest = ompContextReleaseLineageCapabilityDigest(lineageBytes, signature)
	return verified, nil
}

func (verified VerifiedOMPContextReleaseLineage) Coordinates() (OMPContextReleaseLineageV1, bool) {
	if zeroDigest(verified.capabilityDigest) {
		return OMPContextReleaseLineageV1{}, false
	}
	return verified.lineage, true
}

// ExpiresAt returns the trusted receipt boundary carried by this capability.
func (verified VerifiedOMPContextReleaseLineage) ExpiresAt() (time.Time, bool) {
	if zeroDigest(verified.capabilityDigest) || verified.expiresAt.IsZero() {
		return time.Time{}, false
	}
	return verified.expiresAt, true
}

func parseOMPContextReleaseLineage(body []byte) (OMPContextReleaseLineageV1, error) {
	if len(body) == 0 || len(body) > ompContextReleaseLineageMaxBytes {
		return OMPContextReleaseLineageV1{}, errors.New("invalid OMP context release lineage size")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var lineage OMPContextReleaseLineageV1
	if decoder.Decode(&lineage) != nil {
		return lineage, errors.New("invalid OMP context release lineage JSON")
	}
	var trailing any
	if decoder.Decode(&trailing) == nil {
		return lineage, errors.New("trailing OMP context release lineage value")
	}
	canonical, err := CanonicalOMPContextReleaseLineageBytes(lineage)
	if err != nil || !bytes.Equal(canonical, body) {
		return lineage, errors.New("non-canonical OMP context release lineage")
	}
	return lineage, nil
}

func validOMPContextReleaseLineage(value OMPContextReleaseLineageV1) bool {
	return value.SchemaVersion == OMPContextReleaseLineageSchemaV1 && value.Algorithm == "ed25519" &&
		slugPattern.MatchString(value.KeyID) && ompContextReleaseHashPattern.MatchString(value.UpstreamSHA256) &&
		ompContextReleaseHashPattern.MatchString(value.ExecutableSHA256) && slugPattern.MatchString(value.SourceRepository) &&
		ompContextReleaseGitPattern.MatchString(value.SourceCommit) && ompContextReleaseGitPattern.MatchString(value.SourceTree) &&
		slugPattern.MatchString(value.Target) && slugPattern.MatchString(value.Version)
}

func validOMPContextReleaseLineagePolicy(value OMPContextReleaseLineagePolicy) bool {
	return !value.Now.IsZero() && slugPattern.MatchString(value.ExpectedKeyID) &&
		slugPattern.MatchString(value.ExpectedHandoff) && value.MinimumRollbackFloor > 0 &&
		ompContextReleaseHashPattern.MatchString(value.ExpectedUpstreamSHA256) &&
		ompContextReleaseHashPattern.MatchString(value.ExpectedExecutableSHA256) &&
		slugPattern.MatchString(value.ExpectedSourceRepository) && ompContextReleaseGitPattern.MatchString(value.ExpectedSourceCommit) &&
		ompContextReleaseGitPattern.MatchString(value.ExpectedSourceTree) && slugPattern.MatchString(value.ExpectedTarget) &&
		slugPattern.MatchString(value.ExpectedVersion)
}

func matchesOMPContextReleaseLineage(value OMPContextReleaseLineageV1, policy OMPContextReleaseLineagePolicy) bool {
	return value.KeyID == policy.ExpectedKeyID && sameOMPContextReleaseHash(value.UpstreamSHA256, policy.ExpectedUpstreamSHA256) &&
		sameOMPContextReleaseHash(value.ExecutableSHA256, policy.ExpectedExecutableSHA256) && value.SourceRepository == policy.ExpectedSourceRepository &&
		value.SourceCommit == policy.ExpectedSourceCommit && value.SourceTree == policy.ExpectedSourceTree &&
		value.Target == policy.ExpectedTarget && value.Version == policy.ExpectedVersion
}

func ompContextReleaseLineageCapabilityDigest(lineageBytes, signature []byte) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("autopus.omp-context.release-lineage-capability.v1\x00"))
	_, _ = hash.Write(lineageBytes)
	_, _ = hash.Write(signature)
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func sameOMPContextReleaseHash(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
