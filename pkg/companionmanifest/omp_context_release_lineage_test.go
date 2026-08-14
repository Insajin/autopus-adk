package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"
)

func ompContextReleaseLineageFixture() OMPContextReleaseLineageV1 {
	return OMPContextReleaseLineageV1{
		SchemaVersion: OMPContextReleaseLineageSchemaV1,
		KeyID:         "rfc8032-vector-1", Algorithm: "ed25519",
		UpstreamSHA256:   "sha256:" + strings.Repeat("1", 64),
		ExecutableSHA256: "sha256:" + strings.Repeat("2", 64),
		SourceRepository: "Insajin/autopus-adk",
		SourceCommit:     strings.Repeat("3", 40), SourceTree: strings.Repeat("4", 40),
		Target: "darwin-arm64", Version: "v0.50.105",
	}
}

func ompContextReleaseLineagePolicyFixture(lineage OMPContextReleaseLineageV1) OMPContextReleaseLineagePolicy {
	return OMPContextReleaseLineagePolicy{
		Now:           time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		ExpectedKeyID: lineage.KeyID, ExpectedHandoff: "v1", MinimumRollbackFloor: 5069,
		ExpectedUpstreamSHA256:   lineage.UpstreamSHA256,
		ExpectedExecutableSHA256: lineage.ExecutableSHA256,
		ExpectedSourceRepository: lineage.SourceRepository,
		ExpectedSourceCommit:     lineage.SourceCommit, ExpectedSourceTree: lineage.SourceTree,
		ExpectedTarget: lineage.Target, ExpectedVersion: lineage.Version,
	}
}

func signedOMPContextReleaseLineageFixture(t *testing.T) (
	OMPContextReleaseLineageV1, []byte, []byte, TrustedPublicKeyReceipt,
) {
	t.Helper()
	lineage := ompContextReleaseLineageFixture()
	_, privateKey := receiptVectorKeyPair(t)
	body, signature, err := SignCanonicalOMPContextReleaseLineage(lineage, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return lineage, body, signature, trustedPublicKeyReceiptFixture(t)
}

func TestSignCanonicalOMPContextReleaseLineage_ExactBytesAndSignatureRoundTrip(t *testing.T) {
	lineage := ompContextReleaseLineageFixture()
	publicKey, privateKey := receiptVectorKeyPair(t)
	body, signature, err := SignCanonicalOMPContextReleaseLineage(lineage, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	wantBody := []byte(
		`{"schema_version":"autopus.omp_context_release_lineage.v1","key_id":"rfc8032-vector-1","algorithm":"ed25519",` +
			`"upstream_sha256":"sha256:` + strings.Repeat("1", 64) + `","executable_sha256":"sha256:` + strings.Repeat("2", 64) + `",` +
			`"source_repository":"Insajin/autopus-adk","source_commit":"` + strings.Repeat("3", 40) + `","source_tree":"` + strings.Repeat("4", 40) + `",` +
			`"target":"darwin-arm64","version":"v0.50.105"}`,
	)
	if !bytes.Equal(body, wantBody) {
		t.Fatalf("canonical body = %q, want %q", body, wantBody)
	}
	message, err := OMPContextReleaseLineageSigningMessage(wantBody)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(publicKey, message, signature) {
		t.Fatal("signature does not authenticate the canonical domain-separated lineage")
	}
	if ed25519.Verify(publicKey, body, signature) {
		t.Fatal("signature unexpectedly authenticates the lineage without its domain")
	}

	trusted := trustedPublicKeyReceiptFixture(t)
	verified, err := VerifyOMPContextReleaseLineage(
		body, signature, ompContextReleaseLineagePolicyFixture(lineage), trusted,
	)
	if err != nil {
		t.Fatalf("signed lineage did not round-trip through verification: %v", err)
	}
	coordinates, ok := verified.Coordinates()
	if !ok || coordinates != lineage {
		t.Fatalf("round-trip coordinates = %#v, %v", coordinates, ok)
	}

	tampered := lineage
	tampered.Version = "v0.50.100"
	tamperedBody, err := CanonicalOMPContextReleaseLineageBytes(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyOMPContextReleaseLineage(
		tamperedBody, signature, ompContextReleaseLineagePolicyFixture(tampered), trusted,
	); err == nil {
		t.Fatal("signature was accepted for different canonical lineage coordinates")
	}
}

func TestSignCanonicalOMPContextReleaseLineage_RejectsInvalidLineageAndKeyMaterial(t *testing.T) {
	_, privateKey := receiptVectorKeyPair(t)

	lineageCases := []struct {
		name   string
		mutate func(*OMPContextReleaseLineageV1)
	}{
		{name: "schema", mutate: func(lineage *OMPContextReleaseLineageV1) {
			lineage.SchemaVersion = "autopus.omp_context_release_lineage.v2"
		}},
		{name: "algorithm", mutate: func(lineage *OMPContextReleaseLineageV1) {
			lineage.Algorithm = "Ed25519"
		}},
		{name: "upstream digest", mutate: func(lineage *OMPContextReleaseLineageV1) {
			lineage.UpstreamSHA256 = strings.Repeat("1", 64)
		}},
		{name: "source commit", mutate: func(lineage *OMPContextReleaseLineageV1) {
			lineage.SourceCommit = strings.Repeat("G", 40)
		}},
		{name: "version", mutate: func(lineage *OMPContextReleaseLineageV1) {
			lineage.Version = ""
		}},
	}
	for _, test := range lineageCases {
		t.Run("invalid lineage "+test.name, func(t *testing.T) {
			lineage := ompContextReleaseLineageFixture()
			test.mutate(&lineage)
			body, signature, err := SignCanonicalOMPContextReleaseLineage(lineage, privateKey)
			if err == nil {
				t.Fatal("invalid lineage was signed")
			}
			if body != nil || signature != nil {
				t.Fatalf("rejected lineage returned body %q and signature %x", body, signature)
			}
		})
	}

	shortKey := append(ed25519.PrivateKey(nil), privateKey[:ed25519.SeedSize]...)
	inconsistentKey := append(ed25519.PrivateKey(nil), privateKey...)
	inconsistentKey[len(inconsistentKey)-1] ^= 0xff
	keyCases := []struct {
		name string
		key  ed25519.PrivateKey
	}{
		{name: "missing", key: nil},
		{name: "seed only", key: shortKey},
		{name: "inconsistent public suffix", key: inconsistentKey},
	}
	for _, test := range keyCases {
		t.Run("invalid key "+test.name, func(t *testing.T) {
			body, signature, err := SignCanonicalOMPContextReleaseLineage(
				ompContextReleaseLineageFixture(), test.key,
			)
			if err == nil {
				t.Fatal("invalid private key material was accepted")
			}
			if body != nil || signature != nil {
				t.Fatalf("rejected key returned body %q and signature %x", body, signature)
			}
		})
	}
}

func TestVerifyOMPContextReleaseLineage_CoordinatesAndExpiryAreDefensive(t *testing.T) {
	var empty VerifiedOMPContextReleaseLineage
	if coordinates, ok := empty.Coordinates(); ok || coordinates != (OMPContextReleaseLineageV1{}) {
		t.Fatalf("zero capability coordinates = %#v, %v", coordinates, ok)
	}
	if expiresAt, ok := empty.ExpiresAt(); ok || !expiresAt.IsZero() {
		t.Fatalf("zero capability expiry = %s, %v", expiresAt, ok)
	}

	lineage, body, signature, trusted := signedOMPContextReleaseLineageFixture(t)
	verified, err := VerifyOMPContextReleaseLineage(
		body, signature, ompContextReleaseLineagePolicyFixture(lineage), trusted,
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinates, ok := verified.Coordinates()
	if !ok || coordinates != lineage {
		t.Fatalf("coordinates = %#v, %v", coordinates, ok)
	}
	coordinates.Version = "v9.99.99"
	coordinates.SourceCommit = strings.Repeat("f", 40)
	coordinatesAgain, ok := verified.Coordinates()
	if !ok || coordinatesAgain != lineage {
		t.Fatalf("coordinates changed through returned value: %#v, %v", coordinatesAgain, ok)
	}

	expiresAt, ok := verified.ExpiresAt()
	wantExpiry := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	if !ok || expiresAt != wantExpiry {
		t.Fatalf("expires at = %s, %v; want %s", expiresAt, ok, wantExpiry)
	}
	mutatedExpiry := expiresAt.Add(24 * time.Hour)
	expiresAtAgain, ok := verified.ExpiresAt()
	if !ok || expiresAtAgain != wantExpiry || expiresAtAgain == mutatedExpiry {
		t.Fatalf("expiry changed through returned value: got %s, mutated %s, ok=%v", expiresAtAgain, mutatedExpiry, ok)
	}
}

func TestVerifyOMPContextReleaseLineage_RejectsInvalidExpiry(t *testing.T) {
	lineage, body, signature, trusted := signedOMPContextReleaseLineageFixture(t)
	policy := ompContextReleaseLineagePolicyFixture(lineage)

	for _, test := range []struct {
		name      string
		expiresAt string
	}{
		{name: "non UTC spelling", expiresAt: "2026-07-21T00:00:00+00:00"},
		{name: "non canonical fraction", expiresAt: "2026-07-21T00:00:00.000Z"},
		{name: "invalid calendar date", expiresAt: "2026-02-30T00:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			malformed := trusted
			malformed.receipt.ExpiresAt = test.expiresAt
			if _, err := VerifyOMPContextReleaseLineage(body, signature, policy, malformed); err == nil {
				t.Fatalf("invalid receipt expiry %q was accepted", test.expiresAt)
			}
		})
	}

	t.Run("expiry boundary", func(t *testing.T) {
		atExpiry := policy
		atExpiry.Now = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
		if _, err := VerifyOMPContextReleaseLineage(body, signature, atExpiry, trusted); err == nil {
			t.Fatal("lineage capability was accepted at its exclusive expiry boundary")
		}
	})
}

func TestVerifyOMPContextReleaseLineage_TamperAndAuthorityDriftFailClosed(t *testing.T) {
	lineage, body, signature, trusted := signedOMPContextReleaseLineageFixture(t)

	t.Run("wrong signature domain", func(t *testing.T) {
		_, privateKey := receiptVectorKeyPair(t)
		wrong := ed25519.Sign(privateKey, append([]byte("wrong-domain\x00"), body...))
		if _, err := VerifyOMPContextReleaseLineage(
			body, wrong, ompContextReleaseLineagePolicyFixture(lineage), trusted,
		); err == nil {
			t.Fatal("wrong signature domain was accepted")
		}
	})

	for name, mutate := range map[string]func(*OMPContextReleaseLineagePolicy){
		"upstream": func(policy *OMPContextReleaseLineagePolicy) {
			policy.ExpectedUpstreamSHA256 = "sha256:" + strings.Repeat("9", 64)
		},
		"executable": func(policy *OMPContextReleaseLineagePolicy) {
			policy.ExpectedExecutableSHA256 = "sha256:" + strings.Repeat("8", 64)
		},
		"source":         func(policy *OMPContextReleaseLineagePolicy) { policy.ExpectedSourceCommit = strings.Repeat("7", 40) },
		"handoff":        func(policy *OMPContextReleaseLineagePolicy) { policy.ExpectedHandoff = "v2" },
		"rollback floor": func(policy *OMPContextReleaseLineagePolicy) { policy.MinimumRollbackFloor = 5070 },
	} {
		t.Run(name, func(t *testing.T) {
			policy := ompContextReleaseLineagePolicyFixture(lineage)
			mutate(&policy)
			if _, err := VerifyOMPContextReleaseLineage(body, signature, policy, trusted); err == nil {
				t.Fatal("authority drift was accepted")
			}
		})
	}

	t.Run("non canonical", func(t *testing.T) {
		if _, err := VerifyOMPContextReleaseLineage(
			append([]byte(" "), body...), signature, ompContextReleaseLineagePolicyFixture(lineage), trusted,
		); err == nil {
			t.Fatal("non-canonical lineage was accepted")
		}
	})

	t.Run("raw hash", func(t *testing.T) {
		invalid := lineage
		invalid.UpstreamSHA256 = strings.Repeat("1", 64)
		if _, err := CanonicalOMPContextReleaseLineageBytes(invalid); err == nil {
			t.Fatal("raw lineage hash was accepted")
		}
	})
}
