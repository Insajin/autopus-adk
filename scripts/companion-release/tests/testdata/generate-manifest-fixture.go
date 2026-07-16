// Test-only generator for detached-signature mutation fixtures.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type manifest struct {
	SchemaVersion   string `json:"schema_version"`
	ArtifactDigest  string `json:"artifact_digest"`
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	Architecture    string `json:"architecture"`
	BuildProvenance string `json:"build_provenance"`
	Handoff         string `json:"handoff"`
	RollbackFloor   uint64 `json:"rollback_floor"`
	IssuedAt        string `json:"issued_at"`
	ExpiresAt       string `json:"expires_at"`
	KeyID           string `json:"key_id"`
}

type receipt struct {
	SchemaVersion        string `json:"schema_version"`
	KeyID                string `json:"key_id"`
	Algorithm            string `json:"algorithm"`
	PublicKeyEncoding    string `json:"public_key_encoding"`
	PublicKeyBase64      string `json:"public_key_base64"`
	PublicKeySHA256      string `json:"public_key_sha256"`
	IssuedAt             string `json:"issued_at"`
	ExpiresAt            string `json:"expires_at"`
	Handoff              string `json:"handoff"`
	MinimumRollbackFloor uint64 `json:"minimum_rollback_floor"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(path string, data []byte) {
	must(os.WriteFile(path, data, 0o600))
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		panic("fixture output directory is required")
	}
	dir := os.Args[1]
	must(os.MkdirAll(dir, 0o700))
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	must(err)
	artifact := []byte("signed companion artifact\n")
	if len(os.Args) == 3 {
		artifact = []byte(os.Args[2] + "\n")
	}
	artifactSum := sha256.Sum256(artifact)
	manifestBytes, err := json.Marshal(manifest{
		SchemaVersion: "adk-companion-manifest.v1", ArtifactDigest: "sha256:" + hex.EncodeToString(artifactSum[:]),
		Version: "0.50.71", Platform: "darwin", Architecture: "arm64",
		BuildProvenance: "github-actions:Insajin/autopus-adk@fixture", Handoff: "v1",
		RollbackFloor: 5069, IssuedAt: "2026-07-14T12:43:14Z",
		ExpiresAt: "2026-10-12T12:43:14Z", KeyID: "adk-release-2026-q3-b0",
	})
	must(err)
	publicSum := sha256.Sum256(publicKey)
	receiptBytes, err := json.Marshal(receipt{
		SchemaVersion: "adk-companion-public-key-receipt.v1", KeyID: "adk-release-2026-q3-b0",
		Algorithm: "ed25519", PublicKeyEncoding: "base64-raw-32",
		PublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey),
		PublicKeySHA256: "sha256:" + hex.EncodeToString(publicSum[:]),
		IssuedAt:        "2026-07-14T12:43:14Z", ExpiresAt: "2027-07-14T12:43:14Z",
		Handoff: "v1", MinimumRollbackFloor: 5069,
	})
	must(err)
	write(filepath.Join(dir, "auto"), artifact)
	write(filepath.Join(dir, "adk-companion-manifest.json"), manifestBytes)
	write(filepath.Join(dir, "adk-companion-manifest.sig"), ed25519.Sign(privateKey, manifestBytes))
	write(filepath.Join(dir, "public-key-receipt.json"), receiptBytes)
	write(filepath.Join(dir, "public-key-receipt.sig"), ed25519.Sign(privateKey, receiptBytes))
	write(filepath.Join(dir, "signing-key"), []byte(base64.StdEncoding.EncodeToString(privateKey)))
	inconsistentKey := append([]byte(nil), privateKey...)
	inconsistentKey[len(inconsistentKey)-1] ^= 1
	write(filepath.Join(dir, "inconsistent-signing-key"),
		[]byte(base64.StdEncoding.EncodeToString(inconsistentKey)))
	clear(inconsistentKey)
}
