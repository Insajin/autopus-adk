package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyFilesReadsExactRegularFiles(t *testing.T) {
	fixture := newVerifierFixture()
	signed := signDocument(t, fixture.private, fixture.document())
	directory := t.TempDir()
	documentPath := filepath.Join(directory, "adk-channel.json")
	signaturePath := filepath.Join(directory, "adk-channel.json.sig")
	if err := os.WriteFile(documentPath, signed.Document, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, signed.Signature, 0o600); err != nil {
		t.Fatal(err)
	}

	verified, err := VerifyFiles(documentPath, signaturePath, fixture.options)
	if err != nil {
		t.Fatalf("VerifyFiles() error = %v", err)
	}
	if verified.Document.Version != fixture.document().Version {
		t.Fatalf("version = %q", verified.Document.Version)
	}
}

func TestReadSignedFilesRejectsNonRegularAndUnboundedFiles(t *testing.T) {
	directory := t.TempDir()
	validDocumentPath := filepath.Join(directory, "valid.json")
	validSignaturePath := filepath.Join(directory, "valid.sig")
	if err := os.WriteFile(validDocumentPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validSignaturePath, bytes.Repeat([]byte{1}, ed25519.SignatureSize), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		document  func(string) string
		signature func(string) string
	}{
		{
			name: "missing document",
			document: func(path string) string {
				return filepath.Join(directory, "missing.json")
			},
		},
		{
			name: "document directory",
			document: func(path string) string {
				return directory
			},
		},
		{
			name: "empty document",
			document: func(path string) string {
				empty := filepath.Join(directory, "empty.json")
				if err := os.WriteFile(empty, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				return empty
			},
		},
		{
			name: "oversize document",
			document: func(path string) string {
				oversize := filepath.Join(directory, "oversize.json")
				if err := os.WriteFile(oversize, bytes.Repeat([]byte{1}, MaxDocumentBytes+1), 0o600); err != nil {
					t.Fatal(err)
				}
				return oversize
			},
		},
		{
			name: "oversize signature",
			signature: func(path string) string {
				oversize := filepath.Join(directory, "oversize.sig")
				if err := os.WriteFile(oversize, bytes.Repeat([]byte{1}, ed25519.SignatureSize+1), 0o600); err != nil {
					t.Fatal(err)
				}
				return oversize
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			documentPath := validDocumentPath
			signaturePath := validSignaturePath
			if test.document != nil {
				documentPath = test.document(documentPath)
			}
			if test.signature != nil {
				signaturePath = test.signature(signaturePath)
			}
			if _, err := ReadSignedFiles(documentPath, signaturePath); err == nil {
				t.Fatal("ReadSignedFiles() error = nil")
			}
		})
	}
}

func TestVerifyRejectsInvalidPreconditionsAndJSON(t *testing.T) {
	fixture := newVerifierFixture()
	valid := signDocument(t, fixture.private, fixture.document())
	tests := []struct {
		name    string
		signed  SignedBytes
		options Options
	}{
		{name: "empty document", signed: SignedBytes{Signature: valid.Signature}, options: fixture.options},
		{name: "oversize document", signed: SignedBytes{Document: bytes.Repeat([]byte{1}, MaxDocumentBytes+1), Signature: valid.Signature}, options: fixture.options},
		{name: "short signature", signed: SignedBytes{Document: valid.Document, Signature: valid.Signature[:ed25519.SignatureSize-1]}, options: fixture.options},
		{name: "missing key id", signed: valid, options: Options{PublicKeyBase64: fixture.options.PublicKeyBase64, Now: fixture.options.Now}},
		{name: "missing clock", signed: valid, options: Options{PublicKeyBase64: fixture.options.PublicKeyBase64, ExpectedKeyID: testKeyID}},
		{name: "wrong key size", signed: valid, options: Options{PublicKeyBase64: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.PublicKeySize-1)), ExpectedKeyID: testKeyID, Now: fixture.options.Now}},
	}

	malformed := []byte("{")
	tests = append(tests, struct {
		name    string
		signed  SignedBytes
		options Options
	}{
		name: "malformed signed JSON",
		signed: SignedBytes{
			Document:  malformed,
			Signature: ed25519.Sign(fixture.private, malformed),
		},
		options: fixture.options,
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Verify(test.signed, test.options); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}

func TestParseVersionRejectsEmptyComponent(t *testing.T) {
	if _, err := parseVersion("1..3"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("parseVersion() error = %v", err)
	}
}
