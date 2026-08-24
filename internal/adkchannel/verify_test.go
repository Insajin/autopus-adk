package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const testKeyID = "adk-channel-2026-q3-a0"

type verifierFixture struct {
	now     time.Time
	private ed25519.PrivateKey
	options Options
}

func newVerifierFixture() verifierFixture {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	return verifierFixture{
		now:     now,
		private: private,
		options: Options{
			PublicKeyBase64: base64.StdEncoding.EncodeToString(public),
			ExpectedKeyID:   testKeyID,
			Now:             func() time.Time { return now },
		},
	}
}

func (fixture verifierFixture) document() Document {
	return Document{
		SchemaVersion: channelSchema,
		Channel:       channelName,
		Repository:    channelRepository,
		Version:       "1.2.3",
		Tag:           "v1.2.3",
		IssuedAt:      fixture.now.Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt:     fixture.now.Add(time.Hour).Format(time.RFC3339),
		ChannelKeyID:  testKeyID,
	}
}

func signDocument(t *testing.T, private ed25519.PrivateKey, document Document) SignedBytes {
	t.Helper()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return SignedBytes{Document: encoded, Signature: ed25519.Sign(private, encoded)}
}

func TestVerifyAcceptsValidDocumentWithInjectedClock(t *testing.T) {
	fixture := newVerifierFixture()
	verified, err := Verify(signDocument(t, fixture.private, fixture.document()), fixture.options)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.Document.Version != "1.2.3" {
		t.Fatalf("version = %q", verified.Document.Version)
	}
}

func TestVerifyRejectsWrongSignatureBeforeJSON(t *testing.T) {
	fixture := newVerifierFixture()
	signed := signDocument(t, fixture.private, fixture.document())
	signed.Document = []byte("{")

	_, err := Verify(signed, fixture.options)
	if err == nil || !strings.Contains(err.Error(), "signature verification") {
		t.Fatalf("Verify() error = %v, want signature failure", err)
	}
}

func TestVerifyRejectsInvalidTrustedFields(t *testing.T) {
	fixture := newVerifierFixture()
	tests := map[string]func(*Document){
		"wrong key id": func(document *Document) {
			document.ChannelKeyID = "other-key"
		},
		"wrong schema": func(document *Document) {
			document.SchemaVersion = "adk-channel.v2"
		},
		"wrong channel": func(document *Document) {
			document.Channel = "beta"
		},
		"wrong repository": func(document *Document) {
			document.Repository = "Insajin/autopus-desktop"
		},
		"wrong tag": func(document *Document) {
			document.Tag = "v1.2.4"
		},
		"future issued time": func(document *Document) {
			document.IssuedAt = fixture.now.Add(time.Minute).Format(time.RFC3339)
		},
		"expired time": func(document *Document) {
			document.ExpiresAt = fixture.now.Format(time.RFC3339)
		},
		"inverted time window": func(document *Document) {
			document.IssuedAt = fixture.now.Add(2 * time.Hour).Format(time.RFC3339)
			document.ExpiresAt = fixture.now.Add(time.Hour).Format(time.RFC3339)
		},
		"noncanonical time": func(document *Document) {
			document.ExpiresAt = "2026-08-24T13:00:00.000Z"
		},
		"short version": func(document *Document) {
			document.Version = "1.2"
			document.Tag = "v1.2"
		},
		"nonnumeric version": func(document *Document) {
			document.Version = "1.2.3-beta"
			document.Tag = "v1.2.3-beta"
		},
		"overflowing version": func(document *Document) {
			document.Version = "18446744073709551616.2.3"
			document.Tag = "v18446744073709551616.2.3"
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			document := fixture.document()
			mutate(&document)
			if _, err := Verify(signDocument(t, fixture.private, document), fixture.options); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}

func TestVerifyRejectsNoncanonicalPublicKeyBase64(t *testing.T) {
	fixture := newVerifierFixture()
	fixture.options.PublicKeyBase64 += "\n"
	if _, err := Verify(signDocument(t, fixture.private, fixture.document()), fixture.options); err == nil {
		t.Fatal("Verify() error = nil")
	}
}

func TestVerifyUpdateVersionAndExpiryPolicy(t *testing.T) {
	fixture := newVerifierFixture()
	current := signDocument(t, fixture.private, fixture.document())

	withVersion := func(version string, expiry time.Time) SignedBytes {
		document := fixture.document()
		document.Version = version
		document.Tag = "v" + version
		document.ExpiresAt = expiry.Format(time.RFC3339)
		return signDocument(t, fixture.private, document)
	}
	badCurrent := current
	badCurrent.Signature = append([]byte(nil), current.Signature...)
	badCurrent.Signature[0] ^= 0xff
	expiredDocument := fixture.document()
	expiredDocument.IssuedAt = fixture.now.Add(-2 * time.Hour).Format(time.RFC3339)
	expiredDocument.ExpiresAt = fixture.now.Add(-time.Hour).Format(time.RFC3339)
	expiredCurrent := signDocument(t, fixture.private, expiredDocument)
	malformedBytes := []byte("{")
	malformedCurrent := SignedBytes{
		Document:  malformedBytes,
		Signature: ed25519.Sign(fixture.private, malformedBytes),
	}

	tests := []struct {
		name      string
		candidate SignedBytes
		current   *SignedBytes
		wantError bool
	}{
		{name: "first publish", candidate: withVersion("1.2.3", fixture.now.Add(time.Hour)), current: nil},
		{name: "greater version", candidate: withVersion("1.2.4", fixture.now.Add(time.Hour)), current: &current},
		{name: "equal version later expiry", candidate: withVersion("1.2.3", fixture.now.Add(2*time.Hour)), current: &current},
		{name: "expired current greater live candidate", candidate: withVersion("1.2.4", fixture.now.Add(time.Hour)), current: &expiredCurrent},
		{name: "lower version", candidate: withVersion("1.2.2", fixture.now.Add(2*time.Hour)), current: &current, wantError: true},
		{name: "equal version same expiry", candidate: withVersion("1.2.3", fixture.now.Add(time.Hour)), current: &current, wantError: true},
		{name: "invalid current signature", candidate: withVersion("1.2.4", fixture.now.Add(time.Hour)), current: &badCurrent, wantError: true},
		{name: "malformed current document", candidate: withVersion("1.2.4", fixture.now.Add(time.Hour)), current: &malformedCurrent, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerifyUpdate(test.candidate, test.current, fixture.options)
			if test.wantError && err == nil {
				t.Fatal("VerifyUpdate() error = nil")
			}
			if !test.wantError && err != nil {
				t.Fatalf("VerifyUpdate() error = %v", err)
			}
		})
	}
}

func TestVerifyUpdateUsesOneClockInstant(t *testing.T) {
	fixture := newVerifierFixture()
	current := signDocument(t, fixture.private, fixture.document())
	candidateDocument := fixture.document()
	candidateDocument.Version = "1.2.4"
	candidateDocument.Tag = "v1.2.4"
	candidate := signDocument(t, fixture.private, candidateDocument)
	calls := 0
	fixture.options.Now = func() time.Time {
		calls++
		return fixture.now
	}

	if _, err := VerifyUpdate(candidate, &current, fixture.options); err != nil {
		t.Fatalf("VerifyUpdate() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("clock calls = %d, want 1", calls)
	}
}
