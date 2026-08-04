package promptlayer

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	ompContextPromotionAttestationMaxBytesV2 = 16 * 1024
	ompContextPromotionFutureSkewV2          = 5 * time.Minute
	ompContextPromotionMaxTTLV2              = 24 * time.Hour
)

var (
	ErrOMPContextPromotionUnsigned = errors.New("OMP context promotion artifact is unsigned")
	ErrOMPContextPromotionInvalid  = errors.New("OMP context promotion artifact is invalid")
	ErrOMPContextPromotionMismatch = errors.New("OMP context promotion artifact mismatches expected coordinates")
	ErrOMPContextPromotionStale    = errors.New("OMP context promotion artifact is stale")
)

func VerifyOMPContextPromotionArtifactV2(reportBytes, attestationBytes []byte, now time.Time,
	expected OMPContextPromotionExpectationV2) (VerifiedOMPContextPromotion, error) {
	return verifyOMPContextPromotionArtifactV2WithTrust(reportBytes, attestationBytes, now, expected,
		committedOMPContextPromotionPublicKeysV2(), ompContextPromotionRevokedKeysV2)
}

func VerifyOMPContextPromotionHistoricalArtifactV2(reportBytes, attestationBytes []byte,
	expected OMPContextPromotionExpectationV2) (VerifiedOMPContextPromotionHistoricalProof, error) {
	return verifyOMPContextPromotionHistoricalArtifactV2WithTrust(reportBytes, attestationBytes, expected,
		committedOMPContextPromotionPublicKeysV2(), ompContextPromotionRevokedKeysV2)
}

func verifyOMPContextPromotionArtifactV2WithTrust(reportBytes, attestationBytes []byte, now time.Time,
	expected OMPContextPromotionExpectationV2, trusted map[string]ed25519.PublicKey,
	revoked map[string]bool) (VerifiedOMPContextPromotion, error) {
	if !validateOMPContextPromotionExpectationV2(expected) || now.IsZero() {
		return VerifiedOMPContextPromotion{}, fmt.Errorf("%w: expectation or verification time", ErrOMPContextPromotionInvalid)
	}
	if len(bytes.TrimSpace(attestationBytes)) == 0 {
		return VerifiedOMPContextPromotion{}, ErrOMPContextPromotionUnsigned
	}
	attestation, err := decodeOMPContextPromotionAttestationV2(attestationBytes)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	expiresAt, err := verifyOMPContextPromotionSignatureV2(reportBytes, attestation, now.UTC(), trusted, revoked)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	report, err := decodeOMPContextPromotionReportV1(reportBytes)
	if err != nil {
		return VerifiedOMPContextPromotion{}, fmt.Errorf("%w: %v", ErrOMPContextPromotionInvalid, err)
	}
	if report.TrustLane != attestation.TrustLane || !matchesOMPContextPromotionExpectationV2(report, expected) {
		return VerifiedOMPContextPromotion{}, ErrOMPContextPromotionMismatch
	}
	return VerifiedOMPContextPromotion{
		reportDigest: attestation.ReportSHA256, evidenceID: report.EvidenceID, expiresAt: expiresAt,
		producer: report.Producer, candidate: report.Candidate, policyDigest: report.Policy.PolicyDigest,
		runtime: report.Runtime, provider: report.Provider, modelScopeDigest: report.ModelScopeDigest,
	}, nil
}

func verifyOMPContextPromotionHistoricalArtifactV2WithTrust(reportBytes, attestationBytes []byte,
	expected OMPContextPromotionExpectationV2, trusted map[string]ed25519.PublicKey,
	revoked map[string]bool) (VerifiedOMPContextPromotionHistoricalProof, error) {
	if !validateOMPContextPromotionExpectationV2(expected) {
		return VerifiedOMPContextPromotionHistoricalProof{}, fmt.Errorf("%w: expectation", ErrOMPContextPromotionInvalid)
	}
	if len(bytes.TrimSpace(attestationBytes)) == 0 {
		return VerifiedOMPContextPromotionHistoricalProof{}, ErrOMPContextPromotionUnsigned
	}
	attestation, err := decodeOMPContextPromotionAttestationV2(attestationBytes)
	if err != nil {
		return VerifiedOMPContextPromotionHistoricalProof{}, err
	}
	issuedAt, _, expiresAt, err := verifyOMPContextPromotionSignatureAndTimesV2(reportBytes, attestation, trusted, revoked)
	if err != nil {
		return VerifiedOMPContextPromotionHistoricalProof{}, err
	}
	report, err := decodeOMPContextPromotionReportV1(reportBytes)
	if err != nil {
		return VerifiedOMPContextPromotionHistoricalProof{}, fmt.Errorf("%w: %v", ErrOMPContextPromotionInvalid, err)
	}
	if report.TrustLane != attestation.TrustLane || !matchesOMPContextPromotionExpectationV2(report, expected) {
		return VerifiedOMPContextPromotionHistoricalProof{}, ErrOMPContextPromotionMismatch
	}
	return VerifiedOMPContextPromotionHistoricalProof{
		reportDigest: attestation.ReportSHA256, evidenceID: report.EvidenceID, issuedAt: issuedAt, expiresAt: expiresAt,
		producer: report.Producer, candidate: report.Candidate,
	}, nil
}

func decodeOMPContextPromotionAttestationV2(body []byte) (OMPContextPromotionAttestationV2, error) {
	if len(body) == 0 || len(body) > ompContextPromotionAttestationMaxBytesV2 {
		return OMPContextPromotionAttestationV2{}, fmt.Errorf("%w: attestation size", ErrOMPContextPromotionInvalid)
	}
	var attestation OMPContextPromotionAttestationV2
	if err := decodeStrictOMPContextEvidenceV1(body, &attestation); err != nil {
		return attestation, fmt.Errorf("%w: %v", ErrOMPContextPromotionInvalid, err)
	}
	canonical, err := json.Marshal(attestation)
	if err != nil || !bytes.Equal(canonical, body) {
		return attestation, fmt.Errorf("%w: non-canonical attestation", ErrOMPContextPromotionInvalid)
	}
	if attestation.SchemaVersion != OMPContextPromotionAttestationSchemaV2 || attestation.Algorithm != "ed25519" ||
		attestation.KeyID != OMPContextPromotionKeyID2026Q3K1 || attestation.TrustLane != OMPContextPromotionTrustLaneV2 {
		return attestation, fmt.Errorf("%w: attestation identity", ErrOMPContextPromotionInvalid)
	}
	return attestation, nil
}

func verifyOMPContextPromotionSignatureV2(reportBytes []byte, attestation OMPContextPromotionAttestationV2,
	now time.Time, trusted map[string]ed25519.PublicKey, revoked map[string]bool) (time.Time, error) {
	issuedAt, notBefore, expiresAt, err := verifyOMPContextPromotionSignatureAndTimesV2(reportBytes, attestation, trusted, revoked)
	if err != nil {
		return time.Time{}, err
	}
	if now.Before(notBefore.Add(-ompContextPromotionFutureSkewV2)) || issuedAt.After(now.Add(ompContextPromotionFutureSkewV2)) ||
		!now.Before(expiresAt) {
		return time.Time{}, ErrOMPContextPromotionStale
	}
	return expiresAt, nil
}

func verifyOMPContextPromotionSignatureAndTimesV2(reportBytes []byte, attestation OMPContextPromotionAttestationV2,
	trusted map[string]ed25519.PublicKey, revoked map[string]bool) (time.Time, time.Time, time.Time, error) {
	if len(reportBytes) == 0 || len(reportBytes) > ompContextPromotionReportMaxBytesV1 ||
		attestation.ReportSHA256 != promotionSHA256(reportBytes) {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("%w: report digest", ErrOMPContextPromotionInvalid)
	}
	issuedAt, issueErr := parseCanonicalOMPContextPromotionTimeV2(attestation.IssuedAt)
	notBefore, beforeErr := parseCanonicalOMPContextPromotionTimeV2(attestation.NotBefore)
	expiresAt, expiryErr := parseCanonicalOMPContextPromotionTimeV2(attestation.ExpiresAt)
	if issueErr != nil || beforeErr != nil || expiryErr != nil || issuedAt.Before(notBefore) || !issuedAt.Before(expiresAt) ||
		expiresAt.Sub(notBefore) > ompContextPromotionMaxTTLV2 {
		return time.Time{}, time.Time{}, time.Time{}, ErrOMPContextPromotionStale
	}
	publicKey, present := trusted[attestation.KeyID]
	if !present || len(publicKey) != ed25519.PublicKeySize || revoked[attestation.KeyID] {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("%w: signing key", ErrOMPContextPromotionInvalid)
	}
	encoding := base64.StdEncoding.Strict()
	signature, err := encoding.DecodeString(attestation.SignatureBase64)
	if err != nil || len(signature) != ed25519.SignatureSize || encoding.EncodeToString(signature) != attestation.SignatureBase64 {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("%w: signature encoding", ErrOMPContextPromotionInvalid)
	}
	message, err := ompContextPromotionAttestationMessageV2(attestation)
	if err != nil || !ed25519.Verify(publicKey, message, signature) {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("%w: signature", ErrOMPContextPromotionInvalid)
	}
	return issuedAt, notBefore, expiresAt, nil
}

func ompContextPromotionAttestationStatementBytesV2(attestation OMPContextPromotionAttestationV2) ([]byte, error) {
	return json.Marshal(ompContextPromotionAttestationStatementV2{
		SchemaVersion: attestation.SchemaVersion, KeyID: attestation.KeyID, Algorithm: attestation.Algorithm,
		ReportSHA256: attestation.ReportSHA256, IssuedAt: attestation.IssuedAt, NotBefore: attestation.NotBefore,
		ExpiresAt: attestation.ExpiresAt, TrustLane: attestation.TrustLane,
	})
}

func ompContextPromotionAttestationMessageV2(attestation OMPContextPromotionAttestationV2) ([]byte, error) {
	statement, err := ompContextPromotionAttestationStatementBytesV2(attestation)
	if err != nil {
		return nil, err
	}
	return append([]byte(ompContextPromotionAttestationDomainV2), statement...), nil
}

func promotionSHA256(body []byte) string { return canonicalHash(body) }
