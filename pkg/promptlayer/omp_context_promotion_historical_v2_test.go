package promptlayer

import (
	"crypto/ed25519"
	"reflect"
	"testing"
	"time"
)

func TestVerifyOMPContextPromotionHistoricalArtifactV2_AcceptsExpiredImmutableProofWithoutActiveGrant(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	for index := range fixture.report.Observations {
		observation := &fixture.report.Observations[index]
		started, err := time.Parse(time.RFC3339Nano, observation.StartedAt)
		if err != nil {
			t.Fatal(err)
		}
		completed, err := time.Parse(time.RFC3339Nano, observation.CompletedAt)
		if err != nil {
			t.Fatal(err)
		}
		observation.StartedAt = started.Add(-365 * 24 * time.Hour).Format(time.RFC3339Nano)
		observation.CompletedAt = completed.Add(-365 * 24 * time.Hour).Format(time.RFC3339Nano)
	}
	fixture.report.EvidenceID, _ = computeOMPContextPromotionEvidenceIDV1(fixture.report)
	fixture.resign(t)
	fixture.attestation.IssuedAt = "2025-08-04T02:59:00Z"
	fixture.attestation.NotBefore = "2025-08-04T02:59:00Z"
	fixture.attestation.ExpiresAt = "2025-08-04T03:59:00Z"
	fixture.signAttestation(t)

	if _, err := verifyOMPContextPromotionArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes, fixture.now,
		fixture.expectation, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
		t.Fatal("expired proof was accepted for active authority")
	}
	historical, err := verifyOMPContextPromotionHistoricalArtifactV2WithTrust(
		fixture.reportBytes, fixture.attestationBytes, fixture.expectation,
		map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil,
	)
	if err != nil {
		t.Fatalf("verify historical artifact: %v", err)
	}
	if !historical.Valid() || historical.ReportDigest() != fixture.reportDigest ||
		historical.ExpiresAt() != time.Date(2025, 8, 4, 3, 59, 0, 0, time.UTC) {
		t.Fatalf("unexpected historical proof: %#v", historical)
	}
	activeType := reflect.TypeOf(VerifiedOMPContextPromotion{})
	historicalType := reflect.TypeOf(VerifiedOMPContextPromotionHistoricalProof{})
	if historicalType.AssignableTo(activeType) || historicalType.ConvertibleTo(activeType) {
		t.Fatal("historical proof must not be assignable or convertible to an active grant")
	}
}

func TestVerifyOMPContextPromotionHistoricalArtifactV2_StillRejectsInvalidTTLSignatureAndCoordinates(t *testing.T) {
	t.Run("overlong ttl", func(t *testing.T) {
		fixture := newOMPContextPromotionV2Fixture(t)
		fixture.attestation.ExpiresAt = fixture.now.Add(25 * time.Hour).Format(time.RFC3339Nano)
		fixture.signAttestation(t)
		if _, err := verifyOMPContextPromotionHistoricalArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes,
			fixture.expectation, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
			t.Fatal("overlong historical TTL accepted")
		}
	})

	t.Run("static coordinate", func(t *testing.T) {
		fixture := newOMPContextPromotionV2Fixture(t)
		expected := fixture.expectation
		expected.PolicyDigest = promotionSHA256([]byte("other"))
		if _, err := verifyOMPContextPromotionHistoricalArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes,
			expected, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
			t.Fatal("historical coordinate mismatch accepted")
		}
	})

	t.Run("revoked key", func(t *testing.T) {
		fixture := newOMPContextPromotionV2Fixture(t)
		if _, err := verifyOMPContextPromotionHistoricalArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes,
			fixture.expectation, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey},
			map[string]bool{OMPContextPromotionKeyID2026Q3K1: true}); err == nil {
			t.Fatal("revoked historical key accepted")
		}
	})
}

func TestVerifiedOMPContextPromotion_Valid_RechecksExpiryAtUse(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	verified, err := verifyOMPContextPromotionArtifactV2WithTrust(
		fixture.reportBytes,
		fixture.attestationBytes,
		fixture.now,
		fixture.expectation,
		map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !verified.validAt(fixture.now) {
		t.Fatal("fresh grant reported invalid")
	}
	if verified.validAt(fixture.now.Add(time.Hour)) {
		t.Fatal("expired grant remained valid at use")
	}
}
