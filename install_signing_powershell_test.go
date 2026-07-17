package autopusadk_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
)

const (
	windowsFixtureDir = "scripts/release-signing/tests/fixtures/v0.50.73"
	k1Fingerprint     = "e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f"
	k2Fingerprint     = "93d9f681d829f2d0bdba7e1853e6acf9ae2ffd2c760355853218e920c35cc5ff"
	p256SPKIPrefixHex = "3059301306072a8648ce3d020106082a8648ce3d03010703420004"
	ecs1HeaderHex     = "4543533120000000"
)

type signingFixtureMetadata struct {
	ChecksumsSHA256  string `json:"checksums_sha256"`
	SignaturesSHA256 string `json:"signatures_sha256"`
}

type ecdsaSignature struct {
	R *big.Int
	S *big.Int
}

func TestWindowsSigningFixtureMatchesLiveReceipt(t *testing.T) {
	checksums := readTestFile(t, windowsFixtureDir+"/checksums.txt")
	envelope := readTestFile(t, windowsFixtureDir+"/checksums.txt.signatures")
	var metadata signingFixtureMetadata
	if err := json.Unmarshal(readTestFile(t, windowsFixtureDir+"/metadata.json"), &metadata); err != nil {
		t.Fatalf("parse live fixture metadata: %v", err)
	}
	assertSHA256(t, checksums, metadata.ChecksumsSHA256)
	assertSHA256(t, envelope, metadata.SignaturesSHA256)

	lines := strings.Split(string(envelope), "\n")
	if len(lines) != 3 || lines[0] != "AUTOPUS-RELEASE-SIGNATURE-V1" || lines[2] != "" {
		t.Fatalf("unexpected live envelope layout: %q", envelope)
	}
	record := strings.Split(lines[1], "\t")
	if len(record) != 2 || record[0] != k1Fingerprint {
		t.Fatalf("unexpected live signature record: %q", lines[1])
	}
	signatureDER, err := base64.StdEncoding.Strict().DecodeString(record[1])
	if err != nil {
		t.Fatalf("decode live signature: %v", err)
	}

	spki := readPublicSPKI(t, "scripts/release-signing/release-k1-public.pem")
	prefix, _ := hex.DecodeString(p256SPKIPrefixHex)
	if len(spki) != 91 || !bytes.Equal(spki[:len(prefix)], prefix) {
		t.Fatalf("K1 is not the exact 91-byte named P-256 SPKI")
	}
	assertSHA256(t, spki, k1Fingerprint)
	keyAny, err := x509.ParsePKIXPublicKey(spki)
	if err != nil {
		t.Fatalf("parse K1 SPKI: %v", err)
	}
	key, ok := keyAny.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("K1 is not ECDSA")
	}
	var parsed ecdsaSignature
	rest, err := asn1.Unmarshal(signatureDER, &parsed)
	if err != nil || len(rest) != 0 || parsed.R == nil || parsed.S == nil {
		t.Fatalf("parse live DER signature: rest=%d err=%v", len(rest), err)
	}
	digest := sha256.Sum256(checksums)
	if !ecdsa.Verify(key, digest[:], parsed.R, parsed.S) {
		t.Fatal("live v0.50.73 signature does not verify with checked-in K1")
	}
}

func TestWindowsInstallerEmbedsStrictSigningContract(t *testing.T) {
	script := string(readTestFile(t, "install.ps1"))
	k1SPKI := base64.StdEncoding.EncodeToString(readPublicSPKI(t, "scripts/release-signing/release-k1-public.pem"))
	k2SPKI := base64.StdEncoding.EncodeToString(readPublicSPKI(t, "scripts/release-signing/release-k2-public.pem"))
	required := []string{
		"param([switch]$LibraryOnly)",
		"function Convert-SpkiToEcs1", "function Convert-DerSignatureToP1363",
		"function Read-ReleaseSignatureEnvelope", "function Test-ReleaseSignature",
		p256SPKIPrefixHex, ecs1HeaderHex,
		"ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551",
		k1Fingerprint, "2028-07-17", k1SPKI,
		k2Fingerprint, "2030-07-17", k2SPKI,
		"checksums.txt.signatures", "$SigningFloor = \"0.50.73\"", "unsigned_release_not_supported",
		"autopus-adk v$Version installed!", "Autopus-ADK is ready!",
	}
	for _, value := range required {
		if !strings.Contains(script, value) {
			t.Errorf("install.ps1 is missing signing contract %q", value)
		}
	}
	if strings.Contains(strings.ToLower(script), "openssl") {
		t.Fatal("Windows installer must not depend on OpenSSL")
	}
	assertOrdered(t, script,
		"Test-ReleaseSignature $checksumBytes $envelopeBytes",
		"Verify-Checksum \"$TmpDir\\$Archive\" $expected",
		"Expand-Archive",
		"Copy-Item \"$TmpDir\\auto.exe\"",
	)
	assertAtMostLines(t, "install.ps1", 300)
}

func TestWindowsInstallerOracleRunsInPowerShellFiveAndSeven(t *testing.T) {
	workflow := string(readTestFile(t, ".github/workflows/ci.yaml"))
	oraclePath := "scripts/release-signing/tests/windows-installer-signing-test.ps1"
	for _, value := range []string{"powershell.exe", "pwsh.exe", oraclePath} {
		if !strings.Contains(workflow, value) {
			t.Errorf("Windows CI is missing %q", value)
		}
	}
	oracle := string(readTestFile(t, oraclePath))
	for _, value := range []string{
		"Convert-SpkiToEcs1", "Convert-DerSignatureToP1363",
		"Test-ReleaseSignature", "Invoke-WebRequest", "Expand-Archive", "Copy-Item",
	} {
		if !strings.Contains(oracle, value) {
			t.Errorf("PowerShell oracle is missing %q", value)
		}
	}
	assertAtMostLines(t, oraclePath, 300)
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func readPublicSPKI(t *testing.T, path string) []byte {
	t.Helper()
	block, rest := pem.Decode(readTestFile(t, path))
	if block == nil || block.Type != "PUBLIC KEY" || len(bytes.TrimSpace(rest)) != 0 {
		t.Fatalf("%s is not one canonical PUBLIC KEY PEM", path)
	}
	return block.Bytes
}

func assertSHA256(t *testing.T, data []byte, want string) {
	t.Helper()
	digest := sha256.Sum256(data)
	if got := hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("SHA-256 mismatch: got %s want %s", got, want)
	}
}

func assertOrdered(t *testing.T, text string, values ...string) {
	t.Helper()
	position := -1
	for _, value := range values {
		next := strings.Index(text, value)
		if next <= position {
			t.Fatalf("%q is missing or out of order", value)
		}
		position = next
	}
}

func assertAtMostLines(t *testing.T, path string, limit int) {
	t.Helper()
	lines := bytes.Count(readTestFile(t, path), []byte{'\n'})
	if lines > limit {
		t.Fatalf("%s has %d lines; limit is %d", path, lines, limit)
	}
}
