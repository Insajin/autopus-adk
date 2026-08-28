package promptlayer

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSignOMPContextPromotionAttestationV2_RoundTripsAgainstCommittedTrust(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)
	issuedAt := fixture.now.Add(-time.Minute)
	body, err := SignOMPContextPromotionAttestationV2(
		OMPContextPromotionAttestationSignInputV2{
			ReportBytes: fixture.reportBytes,
			IssuedAt:    issuedAt.Format(time.RFC3339Nano),
			NotBefore:   issuedAt.Format(time.RFC3339Nano),
			ExpiresAt:   issuedAt.Add(24 * time.Hour).Format(time.RFC3339Nano),
		},
		fixture.privateKey,
	)
	if err != nil {
		t.Fatalf("sign promotion attestation: %v", err)
	}
	var attestation OMPContextPromotionAttestationV2
	if err := json.Unmarshal(body, &attestation); err != nil {
		t.Fatalf("decode produced attestation: %v", err)
	}
	canonical, err := json.Marshal(attestation)
	if err != nil || !bytes.Equal(body, canonical) {
		t.Fatalf("attestation is not canonical: marshal error=%v", err)
	}
	if attestation.SchemaVersion != OMPContextPromotionAttestationSchemaV2 ||
		attestation.KeyID != OMPContextPromotionKeyID2026Q3K1 || attestation.Algorithm != "ed25519" ||
		attestation.TrustLane != OMPContextPromotionTrustLaneV2 ||
		attestation.ReportSHA256 != promotionSHA256(fixture.reportBytes) {
		t.Fatalf("unexpected fixed attestation statement: %#v", attestation)
	}
	verified, err := verifyOMPContextPromotionArtifactV2At(
		fixture.reportBytes,
		body,
		fixture.now,
		fixture.expectation,
	)
	if err != nil || !verified.validAt(fixture.now) || verified.ReportDigest() != attestation.ReportSHA256 {
		t.Fatalf("produced attestation did not verify: verified=%#v error=%v", verified, err)
	}
}

func TestSignOMPContextPromotionAttestationV2_SelectsMatchingRotationKey(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	k1Seed := sha256.Sum256([]byte("other-production-promotion-key-k1"))
	k2Seed := sha256.Sum256([]byte("other-production-promotion-key-k2"))
	k1Key := ed25519.NewKeyFromSeed(k1Seed[:])
	k2Key := ed25519.NewKeyFromSeed(k2Seed[:])
	originalKeys := ompContextPromotionPublicKeysV2
	originalRevoked := ompContextPromotionRevokedKeysV2
	ompContextPromotionPublicKeysV2 = map[string]ed25519.PublicKey{
		OMPContextPromotionKeyID2026Q3K1: append(ed25519.PublicKey(nil), k1Key[ed25519.SeedSize:]...),
		OMPContextPromotionKeyID2026Q3K2: append(ed25519.PublicKey(nil), k2Key[ed25519.SeedSize:]...),
		OMPContextPromotionKeyID2026Q3K3: append(ed25519.PublicKey(nil), fixture.publicKey...),
	}
	ompContextPromotionRevokedKeysV2 = map[string]bool{}
	t.Cleanup(func() {
		ompContextPromotionPublicKeysV2 = originalKeys
		ompContextPromotionRevokedKeysV2 = originalRevoked
	})
	fixture.expectation.SigningKeyID = OMPContextPromotionKeyID2026Q3K3
	body, err := SignOMPContextPromotionAttestationV2(
		validOMPContextPromotionAttestationSignInputV2(fixture),
		fixture.privateKey,
	)
	if err != nil {
		t.Fatalf("sign with rotation key: %v", err)
	}
	var attestation OMPContextPromotionAttestationV2
	if err := json.Unmarshal(body, &attestation); err != nil {
		t.Fatalf("decode rotation attestation: %v", err)
	}
	if attestation.KeyID != OMPContextPromotionKeyID2026Q3K3 {
		t.Fatalf("rotation key id=%q", attestation.KeyID)
	}
	verified, err := verifyOMPContextPromotionArtifactV2At(
		fixture.reportBytes,
		body,
		fixture.now,
		fixture.expectation,
	)
	if err != nil || !verified.validAt(fixture.now) {
		t.Fatalf("rotation attestation did not verify: verified=%#v error=%v", verified, err)
	}
}

func TestOMPContextPromotionSigningKeyIDV2_RejectsTestKeyForCommittedK3(t *testing.T) {
	seed := sha256.Sum256([]byte("test-only-k3-private-key-mismatch"))
	testPrivateKey := ed25519.NewKeyFromSeed(seed[:])
	testPublicKey := testPrivateKey[ed25519.SeedSize:]
	committedK3 := committedOMPContextPromotionPublicKeysV2()[OMPContextPromotionKeyID2026Q3K3]
	if bytes.Equal(testPublicKey, committedK3) {
		t.Fatal("test-generated public key unexpectedly equals committed K3")
	}
	if keyID, err := OMPContextPromotionSigningKeyIDV2(testPrivateKey); err == nil || keyID != "" {
		t.Fatalf("test-generated private key selected production K3: key_id=%q error=%v", keyID, err)
	}
}

func TestSignOMPContextPromotionAttestationV2_RejectsWrongOrMalformedKey(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)
	input := validOMPContextPromotionAttestationSignInputV2(fixture)
	wrongSeed := sha256.Sum256([]byte("wrong-promotion-signing-key"))
	wrongKey := ed25519.NewKeyFromSeed(wrongSeed[:])
	inconsistentKey := append(ed25519.PrivateKey(nil), fixture.privateKey...)
	inconsistentKey[len(inconsistentKey)-1] ^= 0xff
	for _, privateKey := range []ed25519.PrivateKey{wrongKey, inconsistentKey, fixture.privateKey[:63]} {
		if body, err := SignOMPContextPromotionAttestationV2(input, privateKey); err == nil || body != nil || !errors.Is(err, ErrOMPContextPromotionInvalid) {
			t.Fatalf("invalid key result body=%q error=%v", body, err)
		}
	}
}

func TestSignOMPContextPromotionAttestationV2_RejectsNonCanonicalOrInvalidReport(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)
	duplicate := append([]byte(`{"schema_version":"autopus.omp_context_promotion_report.v1",`), fixture.reportBytes[1:]...)
	wrongLane := replaceJSONField(t, fixture.reportBytes, "trust_lane", "other-lane")
	for _, report := range [][]byte{
		append(append([]byte(nil), fixture.reportBytes...), '\n'),
		duplicate,
		wrongLane,
		[]byte(`{"schema_version":"autopus.omp_context_promotion_report.v1"}`),
	} {
		input := validOMPContextPromotionAttestationSignInputV2(fixture)
		input.ReportBytes = report
		if body, err := SignOMPContextPromotionAttestationV2(input, fixture.privateKey); err == nil || body != nil || !errors.Is(err, ErrOMPContextPromotionInvalid) {
			t.Fatalf("invalid report result body=%q error=%v", body, err)
		}
	}
}

func TestSignOMPContextPromotionAttestationV2_RejectsInvalidTimes(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	withOMPContextPromotionV3FixtureKey(t, fixture.publicKey)
	valid := validOMPContextPromotionAttestationSignInputV2(fixture)
	cases := []OMPContextPromotionAttestationSignInputV2{
		withPromotionSignTime(valid, "issued", "2026-08-04T02:59:00+00:00"),
		withPromotionSignTime(valid, "not-before", fixture.now.Add(time.Minute).Format(time.RFC3339Nano)),
		withPromotionSignTime(valid, "not-before", fixture.now.Add(-10*time.Minute).Format(time.RFC3339Nano)),
		withPromotionSignTime(valid, "expires", fixture.now.Add(-2*time.Minute).Format(time.RFC3339Nano)),
		withPromotionSignTime(valid, "expires", fixture.now.Add(25*time.Hour).Format(time.RFC3339Nano)),
		withPromotionSignTime(valid, "issued", fixture.now.Add(2*time.Hour).Format(time.RFC3339Nano)),
	}
	for _, input := range cases {
		if body, err := SignOMPContextPromotionAttestationV2(input, fixture.privateKey); err == nil || body != nil || !errors.Is(err, ErrOMPContextPromotionStale) {
			t.Fatalf("invalid time result body=%q error=%v input=%#v", body, err, input)
		}
	}
}

func validOMPContextPromotionAttestationSignInputV2(
	fixture *ompContextPromotionV2Fixture,
) OMPContextPromotionAttestationSignInputV2 {
	issuedAt := fixture.now.Add(-time.Minute)
	return OMPContextPromotionAttestationSignInputV2{
		ReportBytes: fixture.reportBytes,
		IssuedAt:    issuedAt.Format(time.RFC3339Nano),
		NotBefore:   issuedAt.Format(time.RFC3339Nano),
		ExpiresAt:   issuedAt.Add(time.Hour).Format(time.RFC3339Nano),
	}
}

func withPromotionSignTime(
	input OMPContextPromotionAttestationSignInputV2,
	field,
	value string,
) OMPContextPromotionAttestationSignInputV2 {
	switch field {
	case "issued":
		input.IssuedAt = value
	case "not-before":
		input.NotBefore = value
	case "expires":
		input.ExpiresAt = value
	}
	return input
}
