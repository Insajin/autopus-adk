package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func signedFixture(t *testing.T) ([]byte, []byte, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey := testKey()
	manifest, signature, err := SignCanonical(testManifest(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, signature, publicKey
}

func TestVerify_ValidSignedArtifact_ReturnsManifest(t *testing.T) {
	manifestBytes, signature, publicKey := signedFixture(t)

	got, err := Verify(manifestBytes, signature, testArtifact, validPolicy(publicKey))
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got != testManifest() {
		t.Fatalf("Verify() = %#v, want %#v", got, testManifest())
	}
}

func TestVerify_InvalidTrustInputs_FailClosed(t *testing.T) {
	manifestBytes, signature, publicKey := signedFixture(t)
	cases := []struct {
		name     string
		manifest []byte
		artifact []byte
		mutate   func(*VerificationPolicy)
	}{
		{name: "manifest tamper", manifest: bytes.Replace(manifestBytes, []byte("0.50.69"), []byte("0.50.70"), 1)},
		{name: "artifact substitution", artifact: []byte("replacement-artifact")},
		{name: "unknown key", mutate: func(p *VerificationPolicy) { p.PinnedKeys = map[string]PinnedKey{} }},
		{name: "revoked key", mutate: func(p *VerificationPolicy) { p.RevokedKeys["release-2026-q3"] = struct{}{} }},
		{name: "expired key", mutate: func(p *VerificationPolicy) {
			p.PinnedKeys["release-2026-q3"] = PinnedKey{PublicKey: publicKey, ExpiresAt: "2026-07-12T00:00:00Z"}
		}},
		{name: "expired manifest", mutate: func(p *VerificationPolicy) { p.Now = time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) }},
		{name: "wrong platform", mutate: func(p *VerificationPolicy) { p.ExpectedPlatform = "linux" }},
		{name: "wrong architecture", mutate: func(p *VerificationPolicy) { p.ExpectedArchitecture = "amd64" }},
		{name: "wrong handoff", mutate: func(p *VerificationPolicy) { p.ExpectedHandoff = "v2" }},
		{name: "wrong expected digest", mutate: func(p *VerificationPolicy) {
			p.ExpectedDigest = "sha256:0a68ed8f821a2348e500777990665b125d911c949839ce8df6cf82806cc1baee"
		}},
		{name: "rollback below floor", mutate: func(p *VerificationPolicy) { p.MinimumRollbackFloor = 5069 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := tc.manifest
			if candidate == nil {
				candidate = manifestBytes
			}
			artifact := tc.artifact
			if artifact == nil {
				artifact = testArtifact
			}
			policy := validPolicy(publicKey)
			if tc.mutate != nil {
				tc.mutate(&policy)
			}
			if _, err := Verify(candidate, signature, artifact, policy); err == nil {
				t.Fatal("Verify() error = nil, want fail-closed rejection")
			}
		})
	}
}

func TestParseStrict_UnknownMissingAndNonCanonicalFields_AreRejected(t *testing.T) {
	canonical, _, _ := signedFixture(t)
	cases := [][]byte{
		bytes.Replace(canonical, []byte(`"key_id":"release-2026-q3"`), []byte(`"key_id":"release-2026-q3","extra":true`), 1),
		bytes.Replace(canonical, []byte(`,"handoff":"v1"`), nil, 1),
		append([]byte(" "), canonical...),
	}
	for _, candidate := range cases {
		if _, err := ParseStrict(candidate); err == nil {
			t.Fatalf("ParseStrict(%q) error = nil", candidate)
		}
	}
}

func TestParseStrict_EmptyOversizeAndTrailingValues_AreRejected(t *testing.T) {
	cases := [][]byte{
		nil,
		make([]byte, maxManifestBytes+1),
		append([]byte(`{"schema_version":"bad"}`), []byte(`{"second":true}`)...),
	}
	for _, candidate := range cases {
		if _, err := ParseStrict(candidate); err == nil {
			t.Fatal("ParseStrict() error = nil")
		}
	}
}

func TestVerify_InvalidSignatureAndFutureManifest_AreRejected(t *testing.T) {
	manifestBytes, signature, publicKey := signedFixture(t)
	policy := validPolicy(publicKey)
	shortSignature := signature[:ed25519.SignatureSize-1]
	if _, err := Verify(manifestBytes, shortSignature, testArtifact, policy); err == nil {
		t.Fatal("Verify(short signature) error = nil")
	}
	policy.Now = time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	if _, err := Verify(manifestBytes, signature, testArtifact, policy); err == nil {
		t.Fatal("Verify(future manifest) error = nil")
	}
}
