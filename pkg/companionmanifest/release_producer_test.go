package companionmanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCompanionReleaseProducer_DarwinTargets_UseExactAssociatedFiles(t *testing.T) {
	for _, architecture := range []string{"amd64", "arm64"} {
		t.Run(architecture, func(t *testing.T) {
			dir := t.TempDir()
			artifactDir := filepath.Join(dir, "auto_darwin_"+architecture)
			if err := os.Mkdir(artifactDir, 0o700); err != nil {
				t.Fatal(err)
			}
			artifact := filepath.Join(artifactDir, "auto")
			if err := os.WriteFile(artifact, []byte("artifact-"+architecture), 0o700); err != nil {
				t.Fatal(err)
			}
			secret := "private-release-material-" + architecture
			keyFile := filepath.Join(dir, "release-key")
			if err := os.WriteFile(keyFile, []byte(secret), 0o600); err != nil {
				t.Fatal(err)
			}
			argsFile := filepath.Join(dir, "signer-args")
			stdinDigestFile := filepath.Join(dir, "stdin-digest")
			signer := writeSignerWrapper(t, dir)

			command := exec.Command("bash", releaseProducerPath(t))
			command.Env = append(companionProducerEnv(
				artifact, architecture, keyFile, signer, argsFile, stdinDigestFile,
			), darwinReleaseToolEnv(t, dir)...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("producer failed: %v\n%s", err, output)
			}
			if strings.Contains(string(output), secret) {
				t.Fatal("producer output leaked private key")
			}
			assertProducerOutputs(t, artifactDir, architecture)
			assertSignerTransport(t, argsFile, stdinDigestFile, artifact, secret)
		})
	}
}

func TestCompanionReleaseProducer_MissingKeyID_FailsWithoutSecretDisclosure(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "auto")
	if err := os.WriteFile(artifact, []byte("artifact"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := "private-release-material"
	keyFile := filepath.Join(dir, "release-key")
	if err := os.WriteFile(keyFile, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := companionProducerEnv(artifact, "arm64", keyFile, writeSignerWrapper(t, dir),
		filepath.Join(dir, "args"), filepath.Join(dir, "digest"))
	environment = removeEnvironment(environment, "COMPANION_KEY_ID")
	command := exec.Command("bash", releaseProducerPath(t))
	command.Env = append(environment, darwinReleaseToolEnv(t, dir)...)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("producer succeeded without key ID")
	}
	if strings.Contains(string(output), secret) {
		t.Fatal("producer failure leaked private key")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "adk-companion-manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("manifest exists after rejected input: %v", statErr)
	}
}

func TestCompanionSignerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COMPANION_SIGNER_HELPER") != "1" {
		return
	}
	args := helperArguments(os.Args)
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(input)
	if err := os.WriteFile(os.Getenv("FAKE_STDIN_DIGEST"), []byte(hex.EncodeToString(sum[:])), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("FAKE_ARGS_OUT"), []byte(strings.Join(args, "\x00")), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := flagValue(t, args, "--manifest-output")
	signature := flagValue(t, args, "--signature-output")
	artifact := flagValue(t, args, "--artifact")
	artifactBytes, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	artifactSum := sha256.Sum256(artifactBytes)
	rollbackFloor, err := strconv.ParseUint(flagValue(t, args, "--rollback-floor"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := CanonicalBytes(Manifest{
		SchemaVersion: SchemaVersion, ArtifactDigest: "sha256:" + hex.EncodeToString(artifactSum[:]),
		Version: flagValue(t, args, "--version"), Platform: flagValue(t, args, "--platform"),
		Architecture:    flagValue(t, args, "--architecture"),
		BuildProvenance: flagValue(t, args, "--build-provenance"),
		Handoff:         flagValue(t, args, "--handoff"), RollbackFloor: rollbackFloor,
		IssuedAt: flagValue(t, args, "--issued-at"), ExpiresAt: flagValue(t, args, "--expires-at"),
		KeyID: flagValue(t, args, "--key-id"),
	})
	if err != nil {
		t.Fatal(err)
	}
	appendDarwinReleaseEvent(t, "manifest_signature")
	if err := os.WriteFile(manifest, manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signature, bytes.Repeat([]byte{0x5a}, 64), 0o600); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FAKE_POST_MANIFEST_MUTATION") == "1" {
		if err := os.WriteFile(artifact, append(artifactBytes, []byte("-mutated")...), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func companionProducerEnv(artifact, architecture, keyFile, signer, argsFile, digestFile string) []string {
	return append(os.Environ(),
		"GO_WANT_COMPANION_SIGNER_HELPER=1",
		"FAKE_ARGS_OUT="+argsFile,
		"FAKE_STDIN_DIGEST="+digestFile,
		"COMPANION_ARTIFACT="+artifact,
		"COMPANION_PLATFORM=darwin",
		"COMPANION_ARCHITECTURE="+architecture,
		"COMPANION_TARGET=darwin_"+architecture,
		"COMPANION_VERSION=0.50.69",
		"COMPANION_BUILD_PROVENANCE=github-actions:Insajin/autopus-adk@abcdef",
		"COMPANION_HANDOFF=v1",
		"COMPANION_ROLLBACK_FLOOR=5069",
		"COMPANION_ISSUED_AT=2026-07-14T00:00:00Z",
		"COMPANION_EXPIRES_AT=2026-07-21T00:00:00Z",
		"COMPANION_KEY_ID=release-2026-q3",
		"COMPANION_SIGNING_KEY_FILE="+keyFile,
		"COMPANION_SIGNER="+signer,
	)
}

func writeSignerWrapper(t *testing.T, dir string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "signer")
	script := "#!/usr/bin/env bash\nexec \"" + executable + "\" -test.run=TestCompanionSignerHelperProcess -- \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func releaseProducerPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", "companion-release", "produce.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func assertProducerOutputs(t *testing.T, dir, architecture string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"adk-companion-darwin-receipt.json",
		"adk-companion-manifest.json",
		"adk-companion-manifest.sig",
		"auto",
	}
	if len(entries) != len(want) {
		t.Fatalf("artifact directory entries = %v", entries)
	}
	for index, entry := range entries {
		if entry.Name() != want[index] {
			t.Fatalf("entry[%d] = %q, want %q", index, entry.Name(), want[index])
		}
	}
	signature, err := os.ReadFile(filepath.Join(dir, "adk-companion-manifest.sig"))
	if err != nil || len(signature) != 64 {
		t.Fatalf("%s signature length = %d, error = %v", architecture, len(signature), err)
	}
}

func assertSignerTransport(t *testing.T, argsFile, digestFile, artifact, secret string) {
	t.Helper()
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), secret) {
		t.Fatal("private key was passed through argv")
	}
	wantArgs := "companion-manifest\x00sign\x00--artifact\x00" + artifact
	if !strings.HasPrefix(string(args), wantArgs) {
		t.Fatalf("signer args = %q", args)
	}
	digest, err := os.ReadFile(digestFile)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256([]byte(secret))
	if string(digest) != hex.EncodeToString(wantDigest[:]) {
		t.Fatal("signer did not receive key bytes through stdin")
	}
}

func helperArguments(args []string) []string {
	for index, arg := range args {
		if arg == "--" {
			return args[index+1:]
		}
	}
	return nil
}

func flagValue(t *testing.T, args []string, name string) string {
	t.Helper()
	for index := 0; index < len(args)-1; index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	t.Fatalf("missing helper flag %s in %v", name, args)
	return ""
}

func removeEnvironment(environment []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		if !strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
