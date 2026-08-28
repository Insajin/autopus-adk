package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"
)

func TestVerifyRotationHistoricalAcceptsOnlyTimeWindowExceptions(t *testing.T) {
	fixture := newRotationFixture(t)
	tests := map[string]func(*RotationDocument){
		"expired": func(document *RotationDocument) {
			document.IssuedAt = fixture.now.Add(-3 * time.Hour).Format(time.RFC3339)
			document.ExpiresAt = fixture.now.Add(-2 * time.Hour).Format(time.RFC3339)
		},
		"future": func(document *RotationDocument) {
			document.IssuedAt = fixture.now.Add(time.Hour).Format(time.RFC3339)
			document.ExpiresAt = fixture.now.Add(2 * time.Hour).Format(time.RFC3339)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			document := fixture.document
			mutate(&document)
			signed := signRotationDocument(t, fixture.private, document)
			if _, err := VerifyRotation(signed, fixture.pins, fixture.options); err == nil {
				t.Fatal("active VerifyRotation() error = nil")
			}
			options := fixture.options
			options.Now = nil
			verified, err := VerifyRotationHistorical(signed, fixture.pins, options)
			if err != nil {
				t.Fatalf("VerifyRotationHistorical() error = %v", err)
			}
			if !bytes.Equal(verified.CanonicalDocument, signed.Document) {
				t.Fatal("historical canonical document differs")
			}
		})
	}
}

func TestVerifyRotationHistoricalPreservesCryptographicAndBoundedContract(t *testing.T) {
	fixture := newRotationFixture(t)
	tests := []struct {
		name   string
		signed func(*testing.T) SignedBytes
	}{
		{
			name: "excessive validity",
			signed: func(t *testing.T) SignedBytes {
				document := fixture.document
				document.IssuedAt = fixture.now.Add(-rotationMaxValidity).Format(time.RFC3339)
				document.ExpiresAt = fixture.now.Add(time.Hour).Format(time.RFC3339)
				return signRotationDocument(t, fixture.private, document)
			},
		},
		{
			name: "inverted validity",
			signed: func(t *testing.T) SignedBytes {
				document := fixture.document
				document.IssuedAt = document.ExpiresAt
				return signRotationDocument(t, fixture.private, document)
			},
		},
		{
			name: "wrong source",
			signed: func(t *testing.T) SignedBytes {
				document := fixture.document
				document.SourceCommit = strings.Repeat("a", 40)
				return signRotationDocument(t, fixture.private, document)
			},
		},
		{
			name: "wrong domain",
			signed: func(t *testing.T) SignedBytes {
				valid := signRotationDocument(t, fixture.private, fixture.document)
				valid.Signature = ed25519.Sign(fixture.private, valid.Document)
				return valid
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyRotationHistorical(test.signed(t), fixture.pins, fixture.options); err == nil {
				t.Fatal("VerifyRotationHistorical() error = nil")
			}
		})
	}
}
