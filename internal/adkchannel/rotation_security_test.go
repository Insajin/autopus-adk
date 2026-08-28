package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRotationRejectsNoncanonicalAndAmbiguousJSON(t *testing.T) {
	fixture := newRotationFixture(t)
	valid := signRotationDocument(t, fixture.private, fixture.document)
	duplicate := bytes.Replace(valid.Document, []byte(`"channel":"stable"`), []byte(`"channel":"stable","channel":"stable"`), 1)
	unknown := bytes.Replace(valid.Document, []byte(`"channel":"stable"`), []byte(`"channel":"stable","unknown":"value"`), 1)
	reordered := bytes.Replace(valid.Document, []byte(`{"schema_version":"adk-key-rotation.v1","channel":"stable"`), []byte(`{"channel":"stable","schema_version":"adk-key-rotation.v1"`), 1)
	tests := map[string][]byte{
		"duplicate field":  duplicate,
		"unknown field":    unknown,
		"leading space":    append([]byte(" "), valid.Document...),
		"trailing newline": append(append([]byte(nil), valid.Document...), '\n'),
		"reordered fields": reordered,
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			message := append(append([]byte(nil), rotationSignatureDomain...), document...)
			signed := SignedBytes{Document: document, Signature: ed25519.Sign(fixture.private, message)}
			if _, err := VerifyRotation(signed, fixture.pins, fixture.options); err == nil {
				t.Fatal("VerifyRotation() error = nil")
			}
		})
	}
}

func TestVerifyRotationRejectsWrongSignatureDomainAndSize(t *testing.T) {
	fixture := newRotationFixture(t)
	valid := signRotationDocument(t, fixture.private, fixture.document)
	rawSignature := ed25519.Sign(fixture.private, valid.Document)
	wrongDomain := ed25519.Sign(fixture.private, append([]byte("autopus.adk-channel.key-rotation.v2\x00"), valid.Document...))
	tests := map[string][]byte{
		"raw document": rawSignature,
		"wrong domain": wrongDomain,
		"short":        valid.Signature[:ed25519.SignatureSize-1],
	}
	for name, signature := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyRotation(SignedBytes{Document: valid.Document, Signature: signature}, fixture.pins, fixture.options); err == nil {
				t.Fatal("VerifyRotation() error = nil")
			}
		})
	}
}

func TestVerifyRotationFilesRejectsNonRegularInputsAndNoncanonicalPins(t *testing.T) {
	fixture := newRotationFixture(t)
	signed := signRotationDocument(t, fixture.private, fixture.document)
	root := t.TempDir()
	documentPath := writeRotationFixtureFile(t, root, "adk-key-rotation-v1.json", signed.Document)
	signaturePath := writeRotationFixtureFile(t, root, "adk-key-rotation-v1.sig", signed.Signature)
	tagPath := writeRotationFixtureFile(t, root, "r2.pub", []byte(fixture.pins.NextTagPublicKey+"\n"))
	fingerprintPath := writeRotationFixtureFile(t, root, "r2.fingerprint", []byte(fixture.pins.NextTagFingerprint+"\n"))
	promotionPath := writeRotationFixtureFile(t, root, "k3.pub", []byte(fixture.pins.NextPromotionPublicKey+"\n"))
	pinFiles := RotationPinFiles{NextTagPublicKeyPath: tagPath, NextTagFingerprintPath: fingerprintPath, NextPromotionPublicKeyPath: promotionPath}

	verified, err := VerifyRotationFiles(documentPath, signaturePath, pinFiles, fixture.options)
	if err != nil || !bytes.Equal(verified.CanonicalDocument, signed.Document) {
		t.Fatalf("VerifyRotationFiles() = (%q, %v)", verified.CanonicalDocument, err)
	}

	t.Run("document symlink", func(t *testing.T) {
		link := filepath.Join(root, "document-link")
		if err := os.Symlink(documentPath, link); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyRotationFiles(link, signaturePath, pinFiles, fixture.options); err == nil {
			t.Fatal("VerifyRotationFiles() error = nil")
		}
	})
	t.Run("pin directory", func(t *testing.T) {
		bad := pinFiles
		bad.NextTagPublicKeyPath = root
		if _, err := VerifyRotationFiles(documentPath, signaturePath, bad, fixture.options); err == nil {
			t.Fatal("VerifyRotationFiles() error = nil")
		}
	})
	t.Run("pin without terminal LF", func(t *testing.T) {
		badPath := writeRotationFixtureFile(t, root, "bad-k3.pub", []byte(fixture.pins.NextPromotionPublicKey))
		bad := pinFiles
		bad.NextPromotionPublicKeyPath = badPath
		if _, err := VerifyRotationFiles(documentPath, signaturePath, bad, fixture.options); err == nil || !strings.Contains(err.Error(), "canonical") {
			t.Fatalf("VerifyRotationFiles() error = %v", err)
		}
	})
}

func TestCheckedInRotationPinsMatchAuthorizedContract(t *testing.T) {
	root := filepath.Join("..", "..", "scripts", "companion-release")
	pins, err := readRotationPins(RotationPinFiles{
		NextTagPublicKeyPath:       filepath.Join(root, "release-tag-signing-2026-q3-r2.pub"),
		NextTagFingerprintPath:     filepath.Join(root, "release-tag-signing-2026-q3-r2.fingerprint"),
		NextPromotionPublicKeyPath: filepath.Join(root, "omp-context-promotion-2026-q3-k3.pub"),
	})
	if err != nil {
		t.Fatalf("readRotationPins() error = %v", err)
	}
	if pins.NextTagPublicKey != rotationNextTagPublicKey ||
		pins.NextTagFingerprint != rotationNextTagFingerprint ||
		pins.NextPromotionPublicKey != rotationNextPromotionPublicKey {
		t.Fatalf("checked-in rotation pins = %+v", pins)
	}
	if err := validateTagSigner(pins.NextTagPublicKey, pins.NextTagFingerprint); err != nil {
		t.Fatalf("checked-in tag signer = %v", err)
	}
	if err := validatePromotionKey(pins.NextPromotionPublicKey, rotationNextPromotionPublicKeySHA256); err != nil {
		t.Fatalf("checked-in promotion key = %v", err)
	}
}

func writeRotationFixtureFile(t *testing.T, root, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
