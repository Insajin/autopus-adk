package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newManifestVerifierFixture(t *testing.T) manifestVerifierFixture {
	t.Helper()
	dir := t.TempDir()
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	artifact := []byte("signed darwin arm64 release artifact v0.50.109")
	artifactDigest := manifestVerifierDigest(artifact)
	publicKeyDigest := manifestVerifierDigest(publicKey)

	receipt := receiptClaims{
		SchemaVersion: receiptSchema, KeyID: "adk-release-2026-q3-b0",
		Algorithm: receiptAlgorithm, PublicKeyEncoding: receiptEncoding,
		PublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey),
		PublicKeySHA256: publicKeyDigest,
		IssuedAt:        "2026-08-01T00:00:00Z", ExpiresAt: "2027-08-01T00:00:00Z",
		Handoff: "v1", MinimumRollbackFloor: 5069,
	}
	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	manifest := manifestClaims{
		SchemaVersion: manifestSchema, ArtifactDigest: artifactDigest, Version: "0.50.109",
		Platform: "darwin", Architecture: "arm64",
		BuildProvenance: "github.com/Insajin/autopus-adk/actions/runs/123",
		Handoff:         "v1", RollbackFloor: 5069,
		IssuedAt: "2026-08-05T00:00:00Z", ExpiresAt: "2027-07-31T00:00:00Z",
		KeyID: "adk-release-2026-q3-b0",
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	artifactPath := filepath.Join(dir, "auto")
	manifestPath := filepath.Join(dir, "companion-manifest.json")
	signaturePath := filepath.Join(dir, "companion-manifest.sig")
	receiptPath := filepath.Join(dir, "public-key-receipt.json")
	receiptSignaturePath := filepath.Join(dir, "public-key-receipt.sig")
	signingKeyPath := filepath.Join(dir, "release-key.b64")
	writeManifestVerifierFile(t, artifactPath, artifact)
	writeManifestVerifierFile(t, manifestPath, manifestBytes)
	writeManifestVerifierFile(t, signaturePath, ed25519.Sign(privateKey, manifestBytes))
	writeManifestVerifierFile(t, receiptPath, receiptBytes)
	writeManifestVerifierFile(t, receiptSignaturePath, ed25519.Sign(privateKey, receiptBytes))
	writeManifestVerifierFile(t, signingKeyPath, []byte(base64.StdEncoding.EncodeToString(privateKey)))

	arguments := []string{
		"--artifact", artifactPath,
		"--manifest", manifestPath,
		"--signature", signaturePath,
		"--receipt", receiptPath,
		"--receipt-signature", receiptSignaturePath,
		"--signing-key", signingKeyPath,
		"--key-id", manifest.KeyID,
		"--version", manifest.Version,
		"--platform", manifest.Platform,
		"--architecture", manifest.Architecture,
		"--handoff", manifest.Handoff,
		"--minimum-rollback-floor", "5069",
		"--manifest-sha256", manifestVerifierDigest(manifestBytes),
	}
	return manifestVerifierFixture{
		arguments: arguments, artifactPath: artifactPath, manifestPath: manifestPath,
		signaturePath: signaturePath, receiptPath: receiptPath, receiptSignaturePath: receiptSignaturePath,
		privateKey: privateKey, publicKeyDigest: publicKeyDigest,
	}
}

func replaceManifestVerifierArgument(t *testing.T, fixture *manifestVerifierFixture,
	currentFlag, replacementFlag, replacementValue string,
) {
	t.Helper()
	for index, argument := range fixture.arguments {
		if argument == currentFlag {
			fixture.arguments[index] = replacementFlag
			fixture.arguments[index+1] = replacementValue
			return
		}
	}
	t.Fatalf("fixture argument %q is missing", currentFlag)
}

func rewriteManifestVerifierReceipt(t *testing.T, fixture *manifestVerifierFixture,
	mutate func(*receiptClaims),
) {
	t.Helper()
	body, err := os.ReadFile(fixture.receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt receiptClaims
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(&receipt)
	body, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	writeManifestVerifierFile(t, fixture.receiptPath, body)
	writeManifestVerifierFile(t, fixture.receiptSignaturePath, ed25519.Sign(fixture.privateKey, body))
}

func rewriteManifestVerifierManifest(t *testing.T, fixture *manifestVerifierFixture,
	mutate func(*manifestClaims),
) {
	t.Helper()
	body, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest manifestClaims
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	mutate(&manifest)
	body, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeManifestVerifierFile(t, fixture.manifestPath, body)
	writeManifestVerifierFile(t, fixture.signaturePath, ed25519.Sign(fixture.privateKey, body))
	for index, argument := range fixture.arguments {
		if argument == "--manifest-sha256" {
			fixture.arguments[index+1] = manifestVerifierDigest(body)
			return
		}
	}
	t.Fatal("fixture manifest digest argument is missing")
}

func manifestVerifierDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func writeManifestVerifierFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
