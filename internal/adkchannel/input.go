package adkchannel

import (
	"crypto/ed25519"
	"fmt"
	"os"
)

func DecodeDispatchInputs(documentBase64, signatureBase64 string) (SignedBytes, error) {
	document, err := decodeCanonicalBase64(documentBase64, MaxDocumentBytes)
	if err != nil {
		return SignedBytes{}, fmt.Errorf("document_base64: %w", err)
	}
	if len(document) == 0 || len(document) > MaxDocumentBytes {
		return SignedBytes{}, fmt.Errorf("document_base64 must decode to between 1 and %d bytes", MaxDocumentBytes)
	}
	signature, err := decodeCanonicalBase64(signatureBase64, ed25519.SignatureSize)
	if err != nil {
		return SignedBytes{}, fmt.Errorf("signature_base64: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return SignedBytes{}, fmt.Errorf("signature_base64 must decode to %d bytes", ed25519.SignatureSize)
	}
	return SignedBytes{Document: document, Signature: signature}, nil
}

func WriteDispatchInputs(documentBase64, signatureBase64, documentPath, signaturePath string) error {
	signed, err := DecodeDispatchInputs(documentBase64, signatureBase64)
	if err != nil {
		return err
	}
	if err := writeExclusive(documentPath, signed.Document); err != nil {
		return fmt.Errorf("write document: %w", err)
	}
	if err := writeExclusive(signaturePath, signed.Signature); err != nil {
		_ = os.Remove(documentPath)
		return fmt.Errorf("write signature: %w", err)
	}
	return nil
}

func writeExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(path)
		return writeErr
	}
	if written != len(data) {
		_ = os.Remove(path)
		return fmt.Errorf("short write")
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}
	return nil
}
