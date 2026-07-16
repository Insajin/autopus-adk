package cli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func TestCompanionManifestCommand_IsRegisteredWithoutPrivateKeyFlags(t *testing.T) {
	root := NewRootCmd()
	command, _, err := root.Find([]string{"companion-manifest", "sign"})
	if err != nil || command.Name() != "sign" {
		t.Fatalf("registered command = %v, error = %v", command, err)
	}
	for _, forbidden := range []string{"private-key", "private-key-file", "signing-key"} {
		if command.Flags().Lookup(forbidden) != nil {
			t.Fatalf("forbidden private-key flag %q is registered", forbidden)
		}
	}
}

func TestCompanionManifestSign_StdinKey_WritesDeterministicSignedFiles(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "auto")
	manifestPath := filepath.Join(dir, "companion.json")
	signaturePath := filepath.Join(dir, "companion.sig")
	if err := os.WriteFile(artifactPath, []byte("artifact-v0.50.69"), 0o700); err != nil {
		t.Fatal(err)
	}
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	encodedKey := base64.StdEncoding.EncodeToString(privateKey)

	firstOut := executeCompanionSign(t, artifactPath, manifestPath, signaturePath, encodedKey)
	firstManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	firstSignature, err := os.ReadFile(signaturePath)
	if err != nil {
		t.Fatal(err)
	}
	secondOut := executeCompanionSign(t, artifactPath, manifestPath, signaturePath, encodedKey)
	secondManifest, _ := os.ReadFile(manifestPath)
	secondSignature, _ := os.ReadFile(signaturePath)
	if firstOut != secondOut || !bytes.Equal(firstManifest, secondManifest) || !bytes.Equal(firstSignature, secondSignature) {
		t.Fatal("repeated signing output is not deterministic")
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)
	policy := companionmanifest.VerificationPolicy{
		PinnedKeys: map[string]companionmanifest.PinnedKey{
			"release-2026-q3": {PublicKey: publicKey, ExpiresAt: "2026-08-01T00:00:00Z"},
		},
		RevokedKeys:          map[string]struct{}{},
		Now:                  time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		ExpectedPlatform:     "darwin",
		ExpectedArchitecture: "arm64",
		ExpectedHandoff:      "v1",
		ExpectedDigest:       "sha256:d482d3c0434ec16e329c74c670c910de7f759fc0292e41007a7bb783e4e8dca5",
		MinimumRollbackFloor: 5068,
	}
	if _, err := companionmanifest.Verify(firstManifest, firstSignature, []byte("artifact-v0.50.69"), policy); err != nil {
		t.Fatalf("produced files do not verify: %v", err)
	}
	for _, path := range []string{manifestPath, signaturePath} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat output %s: %v", filepath.Base(path), statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("output %s mode = %v", filepath.Base(path), info.Mode().Perm())
		}
	}
	manifestSum := sha256.Sum256(firstManifest)
	wantOut := `{"schema_version":"adk-companion-manifest-sign-result.v1","artifact_digest":"sha256:d482d3c0434ec16e329c74c670c910de7f759fc0292e41007a7bb783e4e8dca5","key_id":"release-2026-q3","manifest_sha256":"sha256:` + hex.EncodeToString(manifestSum[:]) + `","signature_encoding":"ed25519-raw","status":"signed"}` + "\n"
	if firstOut != wantOut {
		t.Fatalf("stdout = %q, want %q", firstOut, wantOut)
	}
}

func TestCompanionManifestSign_MissingOrInvalidStdinKey_DoesNotLeakSecret(t *testing.T) {
	command := newCompanionManifestSignCmd()
	secret := strings.Repeat("secret-material", 8)
	command.SetIn(strings.NewReader(secret))
	command.SetOut(&bytes.Buffer{})
	command.SetArgs(validCompanionSignArgs(t.TempDir()))
	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error leaked signing material")
	}
}

func TestCompanionManifestSign_OverlappingArtifactOrOutputs_RejectsWithoutOverwrite(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	encodedKey := base64.StdEncoding.EncodeToString(privateKey)
	cases := []struct {
		name      string
		manifest  func(string) string
		signature func(string) string
	}{
		{name: "manifest is artifact", manifest: func(dir string) string { return filepath.Join(dir, "auto") }, signature: func(dir string) string { return filepath.Join(dir, "signature") }},
		{name: "signature is artifact", manifest: func(dir string) string { return filepath.Join(dir, "manifest") }, signature: func(dir string) string { return filepath.Join(dir, "auto") }},
		{name: "outputs alias", manifest: func(dir string) string { return filepath.Join(dir, "output") }, signature: func(dir string) string { return filepath.Join(dir, ".", "output") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			artifact := filepath.Join(dir, "auto")
			if err := os.WriteFile(artifact, []byte("artifact-v0.50.69"), 0o700); err != nil {
				t.Fatal(err)
			}
			command := newCompanionManifestSignCmd()
			command.SetIn(strings.NewReader(encodedKey))
			command.SetOut(&bytes.Buffer{})
			command.SetArgs(companionSignArgs(artifact, tc.manifest(dir), tc.signature(dir)))
			if err := command.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want path overlap rejection")
			}
			got, err := os.ReadFile(artifact)
			if err != nil || string(got) != "artifact-v0.50.69" {
				t.Fatalf("artifact changed to %q, error = %v", got, err)
			}
		})
	}
}

func TestCompanionManifestSign_HardLinkedOutput_RejectsWithoutArtifactOverwrite(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "auto")
	manifest := filepath.Join(dir, "manifest")
	if err := os.WriteFile(artifact, []byte("artifact-v0.50.69"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(artifact, manifest); err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	command := newCompanionManifestSignCmd()
	command.SetIn(strings.NewReader(base64.StdEncoding.EncodeToString(privateKey)))
	command.SetOut(&bytes.Buffer{})
	command.SetArgs(companionSignArgs(artifact, manifest, filepath.Join(dir, "signature")))
	if err := command.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want file-identity rejection")
	}
	got, err := os.ReadFile(artifact)
	if err != nil || string(got) != "artifact-v0.50.69" {
		t.Fatalf("artifact changed to %q, error = %v", got, err)
	}
}

func TestCompanionManifestSign_MissingRequiredReleaseMetadata_Rejects(t *testing.T) {
	required := []string{
		"--build-provenance", "--handoff", "--rollback-floor",
		"--issued-at", "--expires-at", "--key-id",
	}
	for _, missing := range required {
		t.Run(missing, func(t *testing.T) {
			dir := t.TempDir()
			args := removeCompanionFlag(companionSignArgs(
				filepath.Join(dir, "auto"), filepath.Join(dir, "manifest"), filepath.Join(dir, "signature"),
			), missing)
			if err := os.WriteFile(filepath.Join(dir, "auto"), []byte("artifact-v0.50.69"), 0o700); err != nil {
				t.Fatal(err)
			}
			command := newCompanionManifestSignCmd()
			command.SetIn(strings.NewReader(base64.StdEncoding.EncodeToString(
				ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)),
			)))
			command.SetOut(&bytes.Buffer{})
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatalf("Execute() error = nil without %s", missing)
			}
		})
	}
}

func TestCompanionManifestSign_InvalidRequiredReleaseMetadata_Rejects(t *testing.T) {
	cases := map[string]string{
		"--build-provenance": "build provenance with spaces",
		"--handoff":          "",
		"--rollback-floor":   "not-a-number",
		"--issued-at":        "2026-07-12",
		"--expires-at":       "2026-07-11T00:00:00Z",
		"--key-id":           "invalid key id",
	}
	for flag, invalid := range cases {
		t.Run(flag, func(t *testing.T) {
			dir := t.TempDir()
			args := replaceCompanionFlag(validCompanionSignArgs(dir), flag, invalid)
			command := newCompanionManifestSignCmd()
			command.SetIn(strings.NewReader(base64.StdEncoding.EncodeToString(
				ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)),
			)))
			command.SetOut(&bytes.Buffer{})
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatalf("Execute() error = nil with invalid %s", flag)
			}
		})
	}
}

func executeCompanionSign(t *testing.T, artifact, manifest, signature, encodedKey string) string {
	t.Helper()
	command := newCompanionManifestSignCmd()
	var out bytes.Buffer
	command.SetIn(strings.NewReader(encodedKey + "\n"))
	command.SetOut(&out)
	command.SetArgs(companionSignArgs(artifact, manifest, signature))
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return out.String()
}

func validCompanionSignArgs(dir string) []string {
	artifact := filepath.Join(dir, "auto")
	if err := os.WriteFile(artifact, []byte("artifact-v0.50.69"), 0o700); err != nil {
		panic(err)
	}
	return companionSignArgs(artifact, filepath.Join(dir, "manifest"), filepath.Join(dir, "signature"))
}

func companionSignArgs(artifact, manifest, signature string) []string {
	return []string{
		"--artifact", artifact, "--manifest-output", manifest, "--signature-output", signature,
		"--version", "0.50.69", "--platform", "darwin", "--architecture", "arm64",
		"--build-provenance", "github-actions:release-123@abcdef", "--handoff", "v1",
		"--rollback-floor", "5068", "--issued-at", "2026-07-12T00:00:00Z",
		"--expires-at", "2026-07-19T00:00:00Z", "--key-id", "release-2026-q3",
	}
}

func removeCompanionFlag(args []string, flag string) []string {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == flag {
			return append(append([]string{}, args[:index]...), args[index+2:]...)
		}
	}
	return args
}

func replaceCompanionFlag(args []string, flag, value string) []string {
	replaced := append([]string{}, args...)
	for index := 0; index < len(replaced)-1; index++ {
		if replaced[index] == flag {
			replaced[index+1] = value
			break
		}
	}
	return replaced
}
