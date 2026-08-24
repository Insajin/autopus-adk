package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeDispatchInputsAcceptsCanonicalBoundedBytes(t *testing.T) {
	document := []byte(`{"schema_version":"adk-channel.v1"}`)
	signature := bytes.Repeat([]byte{0x5a}, ed25519.SignatureSize)

	decoded, err := DecodeDispatchInputs(
		base64.StdEncoding.EncodeToString(document),
		base64.StdEncoding.EncodeToString(signature),
	)
	if err != nil {
		t.Fatalf("DecodeDispatchInputs() error = %v", err)
	}
	if !bytes.Equal(decoded.Document, document) || !bytes.Equal(decoded.Signature, signature) {
		t.Fatal("decoded bytes differ from inputs")
	}
}

func TestDecodeDispatchInputsRejectsInvalidEncodingAndBounds(t *testing.T) {
	validDocument := base64.StdEncoding.EncodeToString([]byte("{}"))
	validSignature := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.SignatureSize))
	tests := []struct {
		name      string
		document  string
		signature string
	}{
		{name: "noncanonical document base64", document: validDocument + "\n", signature: validSignature},
		{name: "oversize document", document: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, MaxDocumentBytes+1)), signature: validSignature},
		{name: "short signature", document: validDocument, signature: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.SignatureSize-1))},
		{name: "long signature", document: validDocument, signature: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.SignatureSize+1))},
		{name: "noncanonical signature base64", document: validDocument, signature: validSignature + "\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeDispatchInputs(test.document, test.signature); err == nil {
				t.Fatal("DecodeDispatchInputs() error = nil")
			}
		})
	}
}

func TestWriteDispatchInputsCreatesExclusivePrivateFiles(t *testing.T) {
	directory := t.TempDir()
	documentPath := filepath.Join(directory, "adk-channel.json")
	signaturePath := filepath.Join(directory, "adk-channel.json.sig")
	document := []byte("{}")
	signature := bytes.Repeat([]byte{2}, ed25519.SignatureSize)
	documentBase64 := base64.StdEncoding.EncodeToString(document)
	signatureBase64 := base64.StdEncoding.EncodeToString(signature)

	if err := WriteDispatchInputs(documentBase64, signatureBase64, documentPath, signaturePath); err != nil {
		t.Fatalf("WriteDispatchInputs() error = %v", err)
	}
	for _, path := range []string{documentPath, signaturePath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if permission := info.Mode().Perm(); permission != 0o600 {
			t.Fatalf("%s permission = %#o, want 0600", path, permission)
		}
	}
	if err := WriteDispatchInputs(documentBase64, signatureBase64, documentPath, signaturePath); err == nil {
		t.Fatal("second WriteDispatchInputs() error = nil")
	}
	stored, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, document) {
		t.Fatal("existing document was modified")
	}
}

func TestWriteDispatchInputsRemovesDocumentWhenSignatureWriteFails(t *testing.T) {
	directory := t.TempDir()
	documentPath := filepath.Join(directory, "adk-channel.json")
	signaturePath := filepath.Join(directory, "adk-channel.json.sig")
	if err := os.WriteFile(signaturePath, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := WriteDispatchInputs(
		base64.StdEncoding.EncodeToString([]byte("{}")),
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, ed25519.SignatureSize)),
		documentPath,
		signaturePath,
	)
	if err == nil {
		t.Fatal("WriteDispatchInputs() error = nil")
	}
	if _, statErr := os.Stat(documentPath); !os.IsNotExist(statErr) {
		t.Fatalf("document cleanup error = %v, want not exist", statErr)
	}
}
