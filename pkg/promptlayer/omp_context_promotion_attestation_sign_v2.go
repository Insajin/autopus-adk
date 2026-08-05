package promptlayer

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// OMPContextPromotionAttestationSignInputV2 contains the exact report bytes and
// canonical validity interval bound into a production promotion attestation.
type OMPContextPromotionAttestationSignInputV2 struct {
	ReportBytes []byte
	IssuedAt    string
	NotBefore   string
	ExpiresAt   string
}

// SignOMPContextPromotionAttestationV2 validates a canonical promotion-report-v1
// document and signs its fixed v2 statement with the committed production key.
func SignOMPContextPromotionAttestationV2(
	input OMPContextPromotionAttestationSignInputV2,
	privateKey ed25519.PrivateKey,
) ([]byte, error) {
	report, err := decodeOMPContextPromotionReportV1(input.ReportBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOMPContextPromotionInvalid, err)
	}
	issuedAt, issueErr := parseCanonicalOMPContextPromotionTimeV2(input.IssuedAt)
	notBefore, beforeErr := parseCanonicalOMPContextPromotionTimeV2(input.NotBefore)
	expiresAt, expiryErr := parseCanonicalOMPContextPromotionTimeV2(input.ExpiresAt)
	if issueErr != nil || beforeErr != nil || expiryErr != nil || issuedAt.Before(notBefore) ||
		issuedAt.Sub(notBefore) > ompContextPromotionFutureSkewV2 || !issuedAt.Before(expiresAt) ||
		expiresAt.Sub(issuedAt) > ompContextPromotionMaxTTLV2 {
		return nil, ErrOMPContextPromotionStale
	}
	if err := validateOMPContextPromotionCohortAttestationTimeV2(report, issuedAt); err != nil {
		return nil, err
	}
	normalizedKey, err := validatedOMPContextPromotionSigningKeyV2(privateKey)
	if err != nil {
		return nil, err
	}
	defer clear(normalizedKey)

	attestation := OMPContextPromotionAttestationV2{
		SchemaVersion: OMPContextPromotionAttestationSchemaV2,
		KeyID:         OMPContextPromotionKeyID2026Q3K1,
		Algorithm:     "ed25519",
		ReportSHA256:  promotionSHA256(input.ReportBytes),
		IssuedAt:      input.IssuedAt,
		NotBefore:     input.NotBefore,
		ExpiresAt:     input.ExpiresAt,
		TrustLane:     OMPContextPromotionTrustLaneV2,
	}
	message, err := ompContextPromotionAttestationMessageV2(attestation)
	if err != nil {
		return nil, fmt.Errorf("%w: statement", ErrOMPContextPromotionInvalid)
	}
	attestation.SignatureBase64 = base64.StdEncoding.EncodeToString(ed25519.Sign(normalizedKey, message))
	body, err := json.Marshal(attestation)
	if err != nil {
		return nil, fmt.Errorf("%w: attestation encoding", ErrOMPContextPromotionInvalid)
	}
	return body, nil
}

func validatedOMPContextPromotionSigningKeyV2(privateKey ed25519.PrivateKey) (ed25519.PrivateKey, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: signing key", ErrOMPContextPromotionInvalid)
	}
	seed := privateKey.Seed()
	defer clear(seed)
	normalizedKey := ed25519.NewKeyFromSeed(seed)
	if subtle.ConstantTimeCompare(privateKey, normalizedKey) != 1 {
		clear(normalizedKey)
		return nil, fmt.Errorf("%w: signing key", ErrOMPContextPromotionInvalid)
	}
	trusted := committedOMPContextPromotionPublicKeysV2()
	publicKey, present := trusted[OMPContextPromotionKeyID2026Q3K1]
	derivedPublicKey := normalizedKey[ed25519.SeedSize:]
	if !present || len(publicKey) != ed25519.PublicKeySize ||
		ompContextPromotionRevokedKeysV2[OMPContextPromotionKeyID2026Q3K1] ||
		subtle.ConstantTimeCompare(derivedPublicKey, publicKey) != 1 {
		clear(normalizedKey)
		return nil, fmt.Errorf("%w: signing key", ErrOMPContextPromotionInvalid)
	}
	return normalizedKey, nil
}
