package companionmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const acceptedNotaryID = "123e4567-e89b-42d3-a456-426614174000"

type darwinReleaseReceipt struct {
	SchemaVersion   string `json:"schema_version"`
	ArtifactDigest  string `json:"artifact_digest"`
	ManifestDigest  string `json:"manifest_digest"`
	SignatureDigest string `json:"signature_digest"`
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	Architecture    string `json:"architecture"`
	CodeIdentity    struct {
		Identifier                    string `json:"identifier"`
		TeamID                        string `json:"team_id"`
		DeveloperID                   bool   `json:"developer_id"`
		HardenedRuntime               bool   `json:"hardened_runtime"`
		SecureTimestamp               bool   `json:"secure_timestamp"`
		DesignatedRequirementVerified bool   `json:"designated_requirement_verified"`
	} `json:"code_identity"`
	Notarization struct {
		Status       string `json:"status"`
		SubmissionID string `json:"submission_id"`
	} `json:"notarization"`
}

func assertDarwinReceiptBindsOutputs(t *testing.T, dir, artifact string) {
	t.Helper()
	manifestPath := filepath.Join(dir, "adk-companion-manifest.json")
	signaturePath := filepath.Join(dir, "adk-companion-manifest.sig")
	receiptPath := filepath.Join(dir, "adk-companion-darwin-receipt.json")
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt darwinReleaseReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != "adk-companion-darwin-receipt.v1" ||
		receipt.Version != "0.50.69" || receipt.Platform != "darwin" ||
		receipt.Architecture != "arm64" || receipt.CodeIdentity.Identifier != "co.autopus.adk" ||
		receipt.CodeIdentity.TeamID != "GP2PFA2PUV" || !receipt.CodeIdentity.DeveloperID ||
		!receipt.CodeIdentity.HardenedRuntime || !receipt.CodeIdentity.SecureTimestamp ||
		!receipt.CodeIdentity.DesignatedRequirementVerified ||
		receipt.Notarization.Status != "Accepted" || receipt.Notarization.SubmissionID != acceptedNotaryID {
		t.Fatalf("unexpected Darwin release receipt: %#v", receipt)
	}
	assertReceiptDigest(t, artifact, receipt.ArtifactDigest)
	assertReceiptDigest(t, manifestPath, receipt.ManifestDigest)
	assertReceiptDigest(t, signaturePath, receipt.SignatureDigest)
	var manifest Manifest
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil || json.Unmarshal(manifestBytes, &manifest) != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest.ArtifactDigest != receipt.ArtifactDigest {
		t.Fatalf("receipt artifact digest %q != manifest %q", receipt.ArtifactDigest, manifest.ArtifactDigest)
	}
}

func assertReceiptDigest(t *testing.T, path, got string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("receipt digest for %s = %q, want %q", filepath.Base(path), got, want)
	}
}

func assertNoDarwinReleaseMetadata(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{
		"adk-companion-darwin-receipt.json",
		"adk-companion-manifest.json",
		"adk-companion-manifest.sig",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s exists after fail-closed rejection: %v", name, err)
		}
	}
}
