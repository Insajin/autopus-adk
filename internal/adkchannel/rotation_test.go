package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestVerifyRotationAcceptsCanonicalAuthorizedSidecar(t *testing.T) {
	fixture := newRotationFixture(t)
	signed := signRotationDocument(t, fixture.private, fixture.document)

	verified, err := VerifyRotation(signed, fixture.pins, fixture.options)
	if err != nil {
		t.Fatalf("VerifyRotation() error = %v", err)
	}
	if !bytes.Equal(verified.CanonicalDocument, signed.Document) {
		t.Fatal("verified canonical document differs from signed bytes")
	}
	if verified.Document.SourceCommit != fixture.options.ExpectedSourceCommit {
		t.Fatalf("source commit = %q", verified.Document.SourceCommit)
	}
}

func TestVerifyRotationRejectsWrongContractCoordinates(t *testing.T) {
	fixture := newRotationFixture(t)
	tests := map[string]func(*RotationDocument){
		"schema":               func(document *RotationDocument) { document.SchemaVersion = "adk-key-rotation.v2" },
		"channel":              func(document *RotationDocument) { document.Channel = "beta" },
		"repository":           func(document *RotationDocument) { document.Repository = "other/repository" },
		"bridge tag":           func(document *RotationDocument) { document.BridgeTag = "v0.50.110" },
		"release mode":         func(document *RotationDocument) { document.ReleaseMode = "active" },
		"source commit":        func(document *RotationDocument) { document.SourceCommit = strings.Repeat("a", 40) },
		"source tree":          func(document *RotationDocument) { document.SourceTree = strings.Repeat("b", 40) },
		"channel key":          func(document *RotationDocument) { document.ChannelKeyID = "other-key" },
		"previous fingerprint": func(document *RotationDocument) { document.PreviousTagFingerprint = document.NextTagFingerprint },
		"next tag public":      func(document *RotationDocument) { document.NextTagPublicKey += " comment" },
		"next tag fingerprint": func(document *RotationDocument) { document.NextTagFingerprint = rotationPreviousTagFingerprint },
		"promotion key id":     func(document *RotationDocument) { document.NextPromotionKeyID = "other-promotion-key" },
		"promotion public": func(document *RotationDocument) {
			document.NextPromotionPublicKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.PublicKeySize))
		},
		"promotion sha": func(document *RotationDocument) { document.NextPromotionPublicKeySHA256 = strings.Repeat("0", 64) },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			document := fixture.document
			mutate(&document)
			if _, err := VerifyRotation(signRotationDocument(t, fixture.private, document), fixture.pins, fixture.options); err == nil {
				t.Fatal("VerifyRotation() error = nil")
			}
		})
	}
}

func TestVerifyRotationRejectsMalformedExpectedCoordinatesAndPins(t *testing.T) {
	fixture := newRotationFixture(t)
	signed := signRotationDocument(t, fixture.private, fixture.document)
	tests := []struct {
		name    string
		options RotationOptions
		pins    RotationPins
	}{
		{name: "noncanonical channel public base64", options: withRotationPublicKey(fixture.options, fixture.options.PublicKeyBase64+"\n"), pins: fixture.pins},
		{name: "uppercase source commit", options: withRotationSourceCommit(fixture.options, strings.ToUpper(fixture.options.ExpectedSourceCommit)), pins: fixture.pins},
		{name: "short source tree", options: withRotationSourceTree(fixture.options, fixture.options.ExpectedSourceTree[:39]), pins: fixture.pins},
		{name: "noncanonical key id", options: withRotationKeyID(fixture.options, "test--channel-key"), pins: fixture.pins},
		{name: "noncanonical tag pin", options: fixture.options, pins: RotationPins{NextTagPublicKey: fixture.pins.NextTagPublicKey + " ", NextTagFingerprint: fixture.pins.NextTagFingerprint, NextPromotionPublicKey: fixture.pins.NextPromotionPublicKey}},
		{name: "wrong tag fingerprint pin", options: fixture.options, pins: RotationPins{NextTagPublicKey: fixture.pins.NextTagPublicKey, NextTagFingerprint: rotationPreviousTagFingerprint, NextPromotionPublicKey: fixture.pins.NextPromotionPublicKey}},
		{name: "noncanonical promotion base64 pin", options: fixture.options, pins: RotationPins{NextTagPublicKey: fixture.pins.NextTagPublicKey, NextTagFingerprint: fixture.pins.NextTagFingerprint, NextPromotionPublicKey: fixture.pins.NextPromotionPublicKey + "\n"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyRotation(signed, test.pins, test.options); err == nil {
				t.Fatal("VerifyRotation() error = nil")
			}
		})
	}
}

func TestVerifyRotationEnforcesFreshBoundedValidity(t *testing.T) {
	fixture := newRotationFixture(t)
	tests := map[string]func(*RotationDocument){
		"future": func(document *RotationDocument) {
			document.IssuedAt = fixture.now.Add(time.Second).Format(time.RFC3339)
		},
		"stale":    func(document *RotationDocument) { document.ExpiresAt = fixture.now.Format(time.RFC3339) },
		"inverted": func(document *RotationDocument) { document.IssuedAt = document.ExpiresAt },
		"excessive": func(document *RotationDocument) {
			document.ExpiresAt = fixture.now.Add(rotationMaxValidity).Format(time.RFC3339)
		},
		"fractional": func(document *RotationDocument) { document.IssuedAt = "2026-08-28T11:00:00.000Z" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			document := fixture.document
			mutate(&document)
			if _, err := VerifyRotation(signRotationDocument(t, fixture.private, document), fixture.pins, fixture.options); err == nil {
				t.Fatal("VerifyRotation() error = nil")
			}
		})
	}
}

func withRotationPublicKey(options RotationOptions, value string) RotationOptions {
	options.PublicKeyBase64 = value
	return options
}

func withRotationSourceCommit(options RotationOptions, value string) RotationOptions {
	options.ExpectedSourceCommit = value
	return options
}

func withRotationSourceTree(options RotationOptions, value string) RotationOptions {
	options.ExpectedSourceTree = value
	return options
}

func withRotationKeyID(options RotationOptions, value string) RotationOptions {
	options.ExpectedKeyID = value
	return options
}
