package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type commandFixture struct {
	environment   map[string]string
	document      []byte
	documentPath  string
	signaturePath string
}

func newCommandFixture(t *testing.T) commandFixture {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x33}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	now := time.Now().UTC().Truncate(time.Second)
	document, err := json.Marshal(map[string]string{
		"schema_version": "adk-channel.v1",
		"channel":        "stable",
		"repository":     "Insajin/autopus-adk",
		"version":        "1.2.3",
		"tag":            "v1.2.3",
		"issued_at":      now.Add(-time.Hour).Format(time.RFC3339),
		"expires_at":     now.Add(time.Hour).Format(time.RFC3339),
		"channel_key_id": "test-channel-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(private, document)
	directory := t.TempDir()
	documentPath := filepath.Join(directory, "candidate.json")
	signaturePath := filepath.Join(directory, "candidate.json.sig")
	if err := os.WriteFile(documentPath, document, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, signature, 0o600); err != nil {
		t.Fatal(err)
	}
	return commandFixture{
		environment: map[string]string{
			"ADK_CHANNEL_DOCUMENT_BASE64":    base64.StdEncoding.EncodeToString(document),
			"ADK_CHANNEL_SIGNATURE_BASE64":   base64.StdEncoding.EncodeToString(signature),
			"AUTOPUS_ADK_CHANNEL_PUBLIC_KEY": base64.StdEncoding.EncodeToString(public),
			"AUTOPUS_ADK_CHANNEL_KEY_ID":     "test-channel-key",
		},
		document:      document,
		documentPath:  documentPath,
		signaturePath: signaturePath,
	}
}

func (fixture commandFixture) getenv(name string) string {
	return fixture.environment[name]
}

func TestRunDecodesVerifiesAndAcceptsFirstPublish(t *testing.T) {
	fixture := newCommandFixture(t)
	directory := t.TempDir()
	decodedDocument := filepath.Join(directory, "adk-channel.json")
	decodedSignature := filepath.Join(directory, "adk-channel.json.sig")

	version, err := run([]string{
		"decode",
		"--document-out", decodedDocument,
		"--signature-out", decodedSignature,
	}, fixture.getenv)
	if err != nil || version != "" {
		t.Fatalf("decode = (%q, %v)", version, err)
	}
	storedDocument, err := os.ReadFile(decodedDocument)
	if err != nil || !bytes.Equal(storedDocument, fixture.document) {
		t.Fatalf("decoded document mismatch: %v", err)
	}

	verifyArguments := []string{
		"--document", decodedDocument,
		"--signature", decodedSignature,
	}
	version, err = run(append([]string{"verify"}, verifyArguments...), fixture.getenv)
	if err != nil || version != "1.2.3" {
		t.Fatalf("verify = (%q, %v)", version, err)
	}
	version, err = run(append([]string{"verify-update"}, verifyArguments...), fixture.getenv)
	if err != nil || version != "1.2.3" {
		t.Fatalf("verify-update first publish = (%q, %v)", version, err)
	}
}

func TestRunVerifyUpdateRejectsEqualNonlaterAndInvalidCurrent(t *testing.T) {
	fixture := newCommandFixture(t)
	arguments := []string{
		"verify-update",
		"--document", fixture.documentPath,
		"--signature", fixture.signaturePath,
		"--current-document", fixture.documentPath,
		"--current-signature", fixture.signaturePath,
	}
	if _, err := run(arguments, fixture.getenv); err == nil {
		t.Fatal("equal nonlater run() error = nil")
	}

	arguments[len(arguments)-1] = filepath.Join(t.TempDir(), "missing.sig")
	if _, err := run(arguments, fixture.getenv); err == nil {
		t.Fatal("missing current signature run() error = nil")
	}
}

func TestRunRejectsInvalidCommandContracts(t *testing.T) {
	fixture := newCommandFixture(t)
	tests := [][]string{
		nil,
		{"unknown"},
		{"decode"},
		{"decode", "--unknown"},
		{"verify"},
		{"verify", "--document", fixture.documentPath, "--signature", fixture.signaturePath, "extra"},
		{"verify", "--document", fixture.documentPath, "--signature", fixture.signaturePath, "--current-document", fixture.documentPath, "--current-signature", fixture.signaturePath},
		{"verify-update", "--document", fixture.documentPath, "--signature", fixture.signaturePath, "--current-document", fixture.documentPath},
		{"verify", "--document", filepath.Join(t.TempDir(), "missing"), "--signature", fixture.signaturePath},
	}
	for _, arguments := range tests {
		if _, err := run(arguments, fixture.getenv); err == nil {
			t.Fatalf("run(%q) error = nil", arguments)
		}
	}

	fixture.environment["ADK_CHANNEL_SIGNATURE_BASE64"] = "invalid"
	if _, err := run([]string{
		"decode",
		"--document-out", filepath.Join(t.TempDir(), "document"),
		"--signature-out", filepath.Join(t.TempDir(), "signature"),
	}, fixture.getenv); err == nil {
		t.Fatal("decode invalid base64 run() error = nil")
	}
}
