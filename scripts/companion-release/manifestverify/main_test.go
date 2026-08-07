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
	"strings"
	"testing"
)

type manifestVerifierFixture struct {
	arguments            []string
	artifactPath         string
	manifestPath         string
	signaturePath        string
	receiptPath          string
	receiptSignaturePath string
	privateKey           ed25519.PrivateKey
	publicKeyDigest      string
}

func TestRun_VerifiesCanonicalSignedManifestAndRejectsTampering(t *testing.T) {
	fixture := newManifestVerifierFixture(t)
	if err := run(fixture.arguments); err != nil {
		t.Fatalf("verify canonical signed manifest: %v", err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *manifestVerifierFixture)
	}{
		{
			name: "artifact bytes",
			mutate: func(t *testing.T, fixture *manifestVerifierFixture) {
				writeManifestVerifierFile(t, fixture.artifactPath, []byte("tampered artifact"))
			},
		},
		{
			name: "manifest signature",
			mutate: func(t *testing.T, fixture *manifestVerifierFixture) {
				signature, err := os.ReadFile(fixture.signaturePath)
				if err != nil {
					t.Fatal(err)
				}
				signature[0] ^= 0xff
				writeManifestVerifierFile(t, fixture.signaturePath, signature)
			},
		},
		{
			name: "receipt policy",
			mutate: func(_ *testing.T, fixture *manifestVerifierFixture) {
				for index, argument := range fixture.arguments {
					if argument == "--minimum-rollback-floor" {
						fixture.arguments[index+1] = "5070"
						return
					}
				}
			},
		},
		{
			name: "manifest digest pin",
			mutate: func(_ *testing.T, fixture *manifestVerifierFixture) {
				for index, argument := range fixture.arguments {
					if argument == "--manifest-sha256" {
						fixture.arguments[index+1] = "sha256:" + strings.Repeat("0", 64)
						return
					}
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newManifestVerifierFixture(t)
			test.mutate(t, &fixture)
			if err := run(fixture.arguments); err == nil {
				t.Fatal("tampered release inputs were accepted")
			}
		})
	}
}

func TestRun_RejectsNonCanonicalClaimsAndAmbiguousTrust(t *testing.T) {
	fixture := newManifestVerifierFixture(t)
	manifest, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	nonCanonicalManifest := append(manifest, '\n')
	writeManifestVerifierFile(t, fixture.manifestPath, nonCanonicalManifest)
	writeManifestVerifierFile(t, fixture.signaturePath, ed25519.Sign(fixture.privateKey, nonCanonicalManifest))
	for index, argument := range fixture.arguments {
		if argument == "--manifest-sha256" {
			fixture.arguments[index+1] = manifestVerifierDigest(nonCanonicalManifest)
			break
		}
	}
	if err := run(fixture.arguments); err == nil {
		t.Fatal("non-canonical manifest was accepted")
	}

	fixture = newManifestVerifierFixture(t)
	fixture.arguments = append(fixture.arguments,
		"--public-key-sha256", "sha256:"+strings.Repeat("1", 64),
	)
	if err := run(fixture.arguments); err == nil {
		t.Fatal("ambiguous trust anchors were accepted")
	}

	if err := run(append(newManifestVerifierFixture(t).arguments, "trailing")); err == nil {
		t.Fatal("trailing argument was accepted")
	}
}

func TestRun_VerifiesDigestTrustAndRejectsMalformedSignedClaims(t *testing.T) {
	fixture := newManifestVerifierFixture(t)
	replaceManifestVerifierArgument(t, &fixture, "--signing-key", "--public-key-sha256", fixture.publicKeyDigest)
	if err := run(fixture.arguments); err != nil {
		t.Fatalf("verify with pinned public key digest: %v", err)
	}

	for _, test := range []struct {
		name    string
		wantErr string
		mutate  func(*testing.T, *manifestVerifierFixture)
	}{
		{
			name: "invalid receipt identity", wantErr: "invalid receipt claims",
			mutate: func(t *testing.T, fixture *manifestVerifierFixture) {
				rewriteManifestVerifierReceipt(t, fixture, func(receipt *receiptClaims) {
					receipt.Algorithm = "rsa"
				})
			},
		},
		{
			name: "non-canonical receipt", wantErr: "non-canonical JSON",
			mutate: func(t *testing.T, fixture *manifestVerifierFixture) {
				body, err := os.ReadFile(fixture.receiptPath)
				if err != nil {
					t.Fatal(err)
				}
				body = append(body, '\n')
				writeManifestVerifierFile(t, fixture.receiptPath, body)
				writeManifestVerifierFile(t, fixture.receiptSignaturePath, ed25519.Sign(fixture.privateKey, body))
			},
		},
		{
			name: "invalid manifest provenance", wantErr: "invalid manifest claims",
			mutate: func(t *testing.T, fixture *manifestVerifierFixture) {
				rewriteManifestVerifierManifest(t, fixture, func(manifest *manifestClaims) {
					manifest.BuildProvenance = "https://example.invalid/release?token=secret"
				})
			},
		},
		{
			name: "invalid manifest validity window", wantErr: "invalid validity window",
			mutate: func(t *testing.T, fixture *manifestVerifierFixture) {
				rewriteManifestVerifierManifest(t, fixture, func(manifest *manifestClaims) {
					manifest.ExpiresAt = manifest.IssuedAt
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newManifestVerifierFixture(t)
			test.mutate(t, &fixture)
			err := run(fixture.arguments)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("malformed signed claims result = %v; want %q", err, test.wantErr)
			}
		})
	}
}

func newManifestVerifierFixture(t *testing.T) manifestVerifierFixture {
	t.Helper()
	dir := t.TempDir()
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	artifact := []byte("signed darwin arm64 release artifact v0.50.101")
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
		SchemaVersion: manifestSchema, ArtifactDigest: artifactDigest, Version: "0.50.101",
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
