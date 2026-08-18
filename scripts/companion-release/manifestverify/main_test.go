package main

import (
	"crypto/ed25519"
	"os"
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
