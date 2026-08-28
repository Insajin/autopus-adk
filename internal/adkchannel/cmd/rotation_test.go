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

type rotationCommandFixture struct {
	environment        map[string]string
	document           []byte
	documentPath       string
	signaturePath      string
	tagPublicPath      string
	tagFingerprintPath string
	promotionPath      string
	sourceCommit       string
	sourceTree         string
}

func newRotationCommandFixture(t *testing.T) rotationCommandFixture {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	now := time.Now().UTC().Truncate(time.Second)
	sourceCommit := "c2a50831c925a4b08d55991039ed3b1113392888"
	sourceTree := "cad3d28d8375363c9e4b536c98c39b8d51aaf999"
	tagPublic := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPKdXtl0E+TcLmC94idkTgtM5XUA5UqP9An0vNFp0FlY"
	tagFingerprint := "SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ"
	promotionPublic := "YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM="
	document, err := json.Marshal(struct {
		SchemaVersion                string `json:"schema_version"`
		Channel                      string `json:"channel"`
		Repository                   string `json:"repository"`
		BridgeTag                    string `json:"bridge_tag"`
		ReleaseMode                  string `json:"release_mode"`
		SourceCommit                 string `json:"source_commit"`
		SourceTree                   string `json:"source_tree"`
		IssuedAt                     string `json:"issued_at"`
		ExpiresAt                    string `json:"expires_at"`
		ChannelKeyID                 string `json:"channel_key_id"`
		PreviousTagFingerprint       string `json:"previous_tag_fingerprint"`
		NextTagPublicKey             string `json:"next_tag_public_key"`
		NextTagFingerprint           string `json:"next_tag_fingerprint"`
		NextPromotionKeyID           string `json:"next_promotion_key_id"`
		NextPromotionPublicKey       string `json:"next_promotion_public_key"`
		NextPromotionPublicKeySHA256 string `json:"next_promotion_public_key_sha256"`
	}{
		"adk-key-rotation.v1", "stable", "Insajin/autopus-adk", "v0.50.109",
		"canonical-full-bridge", sourceCommit, sourceTree,
		now.Add(-time.Hour).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339),
		"test-channel-key", "SHA256:bhW+YA+FZ6G4d9Z8BM/eBss6l0I/fcVmV7k986GupK0",
		tagPublic, tagFingerprint, "omp-context-promotion-2026-q3-k3",
		promotionPublic, "2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937",
	})
	if err != nil {
		t.Fatal(err)
	}
	message := append([]byte("autopus.adk-channel.key-rotation.v1\x00"), document...)
	root := t.TempDir()
	return rotationCommandFixture{
		environment: map[string]string{
			"AUTOPUS_ADK_CHANNEL_PUBLIC_KEY": base64.StdEncoding.EncodeToString(public),
			"AUTOPUS_ADK_CHANNEL_KEY_ID":     "test-channel-key",
		},
		document:           document,
		documentPath:       writeCommandFile(t, root, "adk-key-rotation-v1.json", document),
		signaturePath:      writeCommandFile(t, root, "adk-key-rotation-v1.sig", ed25519.Sign(private, message)),
		tagPublicPath:      writeCommandFile(t, root, "r2.pub", []byte(tagPublic+"\n")),
		tagFingerprintPath: writeCommandFile(t, root, "r2.fingerprint", []byte(tagFingerprint+"\n")),
		promotionPath:      writeCommandFile(t, root, "k3.pub", []byte(promotionPublic+"\n")),
		sourceCommit:       sourceCommit,
		sourceTree:         sourceTree,
	}
}

func (fixture rotationCommandFixture) getenv(name string) string {
	return fixture.environment[name]
}

func (fixture rotationCommandFixture) arguments() []string {
	return []string{
		"verify-rotation",
		"--document", fixture.documentPath,
		"--signature", fixture.signaturePath,
		"--source-commit", fixture.sourceCommit,
		"--source-tree", fixture.sourceTree,
		"--next-tag-public-key", fixture.tagPublicPath,
		"--next-tag-fingerprint", fixture.tagFingerprintPath,
		"--next-promotion-public-key", fixture.promotionPath,
	}
}

func TestRunVerifyRotationEmitsOnlyCanonicalDocument(t *testing.T) {
	fixture := newRotationCommandFixture(t)
	output, err := run(fixture.arguments(), fixture.getenv)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	var stdout bytes.Buffer
	writeCommandOutput(&stdout, "verify-rotation", output)
	if !bytes.Equal(stdout.Bytes(), fixture.document) {
		t.Fatalf("command output = %q", stdout.Bytes())
	}
}

func TestRunVerifyRotationHistoricalEmitsOnlyCanonicalDocument(t *testing.T) {
	fixture := newRotationCommandFixture(t)
	arguments := fixture.arguments()
	arguments[0] = "verify-rotation-historical"
	output, err := run(arguments, fixture.getenv)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	var stdout bytes.Buffer
	writeCommandOutput(&stdout, "verify-rotation-historical", output)
	if !bytes.Equal(stdout.Bytes(), fixture.document) {
		t.Fatalf("historical command output = %q", stdout.Bytes())
	}
}

func TestWriteCommandOutputPreservesV1Newline(t *testing.T) {
	var stdout bytes.Buffer
	writeCommandOutput(&stdout, "verify", "1.2.3")
	if stdout.String() != "1.2.3\n" {
		t.Fatalf("v1 command output = %q", stdout.String())
	}
}

func TestRunVerifyRotationRejectsMissingPinAndWrongSource(t *testing.T) {
	fixture := newRotationCommandFixture(t)
	missingPin := fixture.arguments()[:len(fixture.arguments())-2]
	if _, err := run(missingPin, fixture.getenv); err == nil {
		t.Fatal("missing pin run() error = nil")
	}
	wrongSource := fixture.arguments()
	wrongSource[6] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := run(wrongSource, fixture.getenv); err == nil {
		t.Fatal("wrong source run() error = nil")
	}
}

func writeCommandFile(t *testing.T, root, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
