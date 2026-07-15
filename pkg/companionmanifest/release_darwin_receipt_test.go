package companionmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDarwinReleaseEnvironment_MissingTrustInputsFailClosed(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "auto")
	keyFile := filepath.Join(dir, "release-key")
	if err := os.WriteFile(artifact, []byte("auto"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("private-release-material"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := companionProducerEnv(
		artifact, "arm64", keyFile, writeSignerWrapper(t, dir),
		filepath.Join(dir, "args"), filepath.Join(dir, "digest"),
	)
	environment = append(environment, darwinReleaseToolEnv(t, dir)...)
	for _, missing := range []string{
		"COMPANION_SIGNING_KEY_FILE", "COMPANION_SIGNER", "COMPANION_KEY_ID",
		"COMPANION_HANDOFF", "COMPANION_ROLLBACK_FLOOR", "COMPANION_ISSUED_AT",
		"COMPANION_EXPIRES_AT", "APPLE_SIGNING_IDENTITY", "APPLE_API_KEY",
		"APPLE_API_ISSUER", "APPLE_API_KEY_PATH",
	} {
		t.Run(missing, func(t *testing.T) {
			command := exec.Command("bash", releaseEnvironmentValidatorPath(t))
			command.Env = removeEnvironment(environment, missing)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("validator accepted missing %s", missing)
			}
			if strings.Contains(string(output), "private-release-material") {
				t.Fatal("validator output leaked signing material")
			}
		})
	}
}

func releaseEnvironmentValidatorPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", "companion-release", "validate-environment.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
