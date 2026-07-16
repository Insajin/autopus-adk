package companionmanifest

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"
)

var testArtifact = []byte("artifact-v0.50.69")

func testManifest() Manifest {
	return Manifest{
		SchemaVersion:   SchemaVersion,
		ArtifactDigest:  "sha256:d482d3c0434ec16e329c74c670c910de7f759fc0292e41007a7bb783e4e8dca5",
		Version:         "0.50.69",
		Platform:        "darwin",
		Architecture:    "arm64",
		BuildProvenance: "github-actions:release-123@abcdef",
		Handoff:         "v1",
		RollbackFloor:   5068,
		IssuedAt:        "2026-07-12T00:00:00Z",
		ExpiresAt:       "2026-07-19T00:00:00Z",
		KeyID:           "release-2026-q3",
	}
}

func testKey() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	return privateKey.Public().(ed25519.PublicKey), privateKey
}

func TestCanonicalBytes_GoldenContract_MatchesExactBytes(t *testing.T) {
	want := `{"schema_version":"adk-companion-manifest.v1","artifact_digest":"sha256:d482d3c0434ec16e329c74c670c910de7f759fc0292e41007a7bb783e4e8dca5","version":"0.50.69","platform":"darwin","architecture":"arm64","build_provenance":"github-actions:release-123@abcdef","handoff":"v1","rollback_floor":5068,"issued_at":"2026-07-12T00:00:00Z","expires_at":"2026-07-19T00:00:00Z","key_id":"release-2026-q3"}`

	got, err := CanonicalBytes(testManifest())
	if err != nil {
		t.Fatalf("CanonicalBytes() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("CanonicalBytes() = %q, want %q", got, want)
	}
}

func TestSignCanonical_GoldenSignature_IsStable(t *testing.T) {
	_, privateKey := testKey()
	_, signature, err := SignCanonical(testManifest(), privateKey)
	if err != nil {
		t.Fatalf("SignCanonical() error = %v", err)
	}
	const want = "Y1YjfIpb4zJ1onvRJWcy/6nztHKkHcUz4Ot/b3lLLMpv0mo3Vo8TPxsO4PxPRFVDR7/akFeUQA3rRmatWEkbBw=="
	if got := base64.StdEncoding.EncodeToString(signature); got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestSignCanonical_Repeated100Times_IsDeterministic(t *testing.T) {
	_, privateKey := testKey()
	firstManifest, firstSignature, err := SignCanonical(testManifest(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		manifest, signature, signErr := SignCanonical(testManifest(), privateKey)
		if signErr != nil {
			t.Fatalf("iteration %d: %v", i, signErr)
		}
		if string(manifest) != string(firstManifest) || string(signature) != string(firstSignature) {
			t.Fatalf("iteration %d produced nondeterministic output", i)
		}
	}
}

func TestManifestValidation_MalformedFields_AreRejected(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "schema", mutate: func(m *Manifest) { m.SchemaVersion = "future.v2" }},
		{name: "digest", mutate: func(m *Manifest) { m.ArtifactDigest = "sha256:ABC" }},
		{name: "version", mutate: func(m *Manifest) { m.Version = "" }},
		{name: "platform", mutate: func(m *Manifest) { m.Platform = "darwin arm" }},
		{name: "architecture", mutate: func(m *Manifest) { m.Architecture = "" }},
		{name: "handoff", mutate: func(m *Manifest) { m.Handoff = "" }},
		{name: "key id", mutate: func(m *Manifest) { m.KeyID = "bad key" }},
		{name: "empty provenance", mutate: func(m *Manifest) { m.BuildProvenance = "" }},
		{name: "control provenance", mutate: func(m *Manifest) { m.BuildProvenance = "build\nsecret" }},
		{name: "oversize provenance", mutate: func(m *Manifest) { m.BuildProvenance = string(make([]byte, 513)) }},
		{name: "issued timezone", mutate: func(m *Manifest) { m.IssuedAt = "2026-07-12T09:00:00+09:00" }},
		{name: "expiry before issue", mutate: func(m *Manifest) { m.ExpiresAt = m.IssuedAt }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := testManifest()
			tc.mutate(&manifest)
			if _, err := CanonicalBytes(manifest); err == nil {
				t.Fatal("CanonicalBytes() error = nil")
			}
		})
	}
}

func TestSignCanonical_InvalidPrivateKey_IsRejected(t *testing.T) {
	if _, _, err := SignCanonical(testManifest(), ed25519.PrivateKey("short")); err == nil {
		t.Fatal("SignCanonical() error = nil")
	}
}

func TestSignCanonical_InconsistentFullLengthPrivateKey_IsRejected(t *testing.T) {
	_, privateKey := testKey()
	inconsistent := append(ed25519.PrivateKey(nil), privateKey...)
	inconsistent[len(inconsistent)-1] ^= 1
	if _, _, err := SignCanonical(testManifest(), inconsistent); err == nil {
		t.Fatal("SignCanonical() accepted an inconsistent 64-byte private key")
	}
}

func validPolicy(publicKey ed25519.PublicKey) VerificationPolicy {
	return VerificationPolicy{
		PinnedKeys: map[string]PinnedKey{
			"release-2026-q3": {PublicKey: publicKey, ExpiresAt: "2026-08-01T00:00:00Z"},
		},
		RevokedKeys:          map[string]struct{}{},
		Now:                  time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		MinimumRollbackFloor: 5068,
		ExpectedPlatform:     "darwin",
		ExpectedArchitecture: "arm64",
		ExpectedHandoff:      "v1",
		ExpectedDigest:       testManifest().ArtifactDigest,
	}
}
