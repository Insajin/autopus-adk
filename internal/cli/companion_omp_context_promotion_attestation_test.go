package cli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompanionOMPContextPromotionAttestationCommand_IsRegisteredWithoutSecretFlags(t *testing.T) {
	root := NewRootCmd()
	command, _, err := root.Find([]string{"companion-manifest", "omp-context-promotion-attestation"})
	if err != nil || command.Name() != "omp-context-promotion-attestation" {
		t.Fatalf("registered command=%v error=%v", command, err)
	}
	for _, required := range []string{"report", "issued-at", "not-before", "expires-at", "valid-for", "output"} {
		flag := command.Flags().Lookup(required)
		if flag == nil {
			t.Fatalf("required flag %q is not registered", required)
		}
	}
	for _, forbidden := range []string{
		"key", "key-file", "private-key", "private-key-file", "signing-key",
		"expected-key-id", "expected-signing-key-id",
	} {
		if command.Flags().Lookup(forbidden) != nil {
			t.Fatalf("secret-bearing flag %q is registered", forbidden)
		}
	}
}
func TestCompanionOMPContextPromotionAttestation_ValidityDurationUsesOnePreciseTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 500_000_123, time.UTC)
	issuedAt, notBefore, expiresAt, err := resolveOMPContextPromotionAttestationWindow(
		companionOMPContextPromotionAttestationOptions{validFor: 24 * time.Hour},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if issuedAt != "2026-08-04T12:00:00.500000123Z" || notBefore != issuedAt ||
		expiresAt != "2026-08-05T12:00:00.500000123Z" {
		t.Fatalf("validity window=%q %q %q", issuedAt, notBefore, expiresAt)
	}
}

func TestCompanionOMPContextPromotionAttestation_RejectsExistingOrSymlinkOutput(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		name := "existing"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			report := filepath.Join(dir, "omp-context-promotion-report.v1.json")
			output := filepath.Join(dir, "omp-context-promotion-attestation.v2.json")
			protected := filepath.Join(dir, "protected")
			if err := os.WriteFile(report, []byte(`{}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(protected, []byte("protected-bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			if symlink {
				if err := os.Symlink(protected, output); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(output, []byte("existing-bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			result := executeOMPContextPromotionAttestationCommand(report, output, validTestPrivateKeyBase64())
			if result.err == nil {
				t.Fatal("existing output was accepted")
			}
			got, err := os.ReadFile(protected)
			if err != nil || string(got) != "protected-bytes" {
				t.Fatalf("protected file changed to %q error=%v", got, err)
			}
			if !symlink {
				got, err = os.ReadFile(output)
				if err != nil || string(got) != "existing-bytes" {
					t.Fatalf("existing output changed to %q error=%v", got, err)
				}
			}
		})
	}
}

func TestCompanionOMPContextPromotionAttestation_RejectsSymlinkReportAndDuplicateJSON(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "report-target")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkReport := filepath.Join(dir, "report-link")
	if err := os.Symlink(target, symlinkReport); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "symlink-output")
	if result := executeOMPContextPromotionAttestationCommand(
		symlinkReport,
		output,
		validTestPrivateKeyBase64(),
	); result.err == nil {
		t.Fatal("symlink report was accepted")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("symlink-report failure created output: %v", err)
	}

	duplicateReport := filepath.Join(dir, "duplicate-report.json")
	if err := os.WriteFile(
		duplicateReport,
		[]byte(`{"schema_version":"first","schema_version":"second"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	duplicateOutput := filepath.Join(dir, "duplicate-output")
	if result := executeOMPContextPromotionAttestationCommand(
		duplicateReport,
		duplicateOutput,
		validTestPrivateKeyBase64(),
	); result.err == nil {
		t.Fatal("duplicate report JSON was accepted")
	}
	if _, err := os.Lstat(duplicateOutput); !os.IsNotExist(err) {
		t.Fatalf("duplicate-report failure created output: %v", err)
	}
}

func TestCompanionOMPContextPromotionAttestation_FailureDoesNotLeakSigningKey(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "invalid-report.json")
	output := filepath.Join(dir, "attestation.json")
	if err := os.WriteFile(report, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	seed := sha256.Sum256([]byte("sensitive-cli-promotion-key"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	encoded := base64.StdEncoding.EncodeToString(privateKey)
	result := executeOMPContextPromotionAttestationCommand(report, output, encoded)
	if result.err == nil {
		t.Fatal("invalid report unexpectedly signed")
	}
	surface := append(append([]byte(result.stdout), result.stderr...), []byte(result.err.Error())...)
	for _, secret := range [][]byte{seed[:], privateKey, []byte(encoded)} {
		if bytes.Contains(surface, secret) {
			t.Fatal("command failure leaked signing key material")
		}
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("failed signer created output: %v", err)
	}
}

func TestWriteNewPrivateOMPContextPromotionAttestation_IsAtomicPrivateAndNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "omp-context-promotion-attestation.v2.json")
	body := []byte(`{"canonical":"attestation"}`)
	if err := writeNewPrivateOMPContextPromotionAttestation(path, body); err != nil {
		t.Fatalf("write private attestation: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("attestation bytes=%q error=%v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("attestation mode=%v error=%v", info.Mode(), err)
	}
	if err := writeNewPrivateOMPContextPromotionAttestation(path, []byte("replacement")); err == nil {
		t.Fatal("writer overwrote an existing attestation")
	}
	got, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("attestation changed to %q error=%v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || strings.Contains(entries[0].Name(), ".stage-") {
		t.Fatalf("unexpected output directory entries=%v error=%v", entries, err)
	}
}

type ompContextPromotionAttestationCommandResult struct {
	stdout string
	stderr string
	err    error
}

func executeOMPContextPromotionAttestationCommand(
	report,
	output,
	encodedKey string,
) ompContextPromotionAttestationCommandResult {
	command := newCompanionOMPContextPromotionAttestationCmd()
	var stdout, stderr bytes.Buffer
	command.SetIn(strings.NewReader(encodedKey + "\n"))
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{
		"--report", report,
		"--issued-at", "2026-08-04T02:59:00Z",
		"--not-before", "2026-08-04T02:59:00Z",
		"--expires-at", "2026-08-04T03:59:00Z",
		"--output", output,
	})
	return ompContextPromotionAttestationCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    command.Execute(),
	}
}

func validTestPrivateKeyBase64() string {
	seed := sha256.Sum256([]byte("cli-promotion-key"))
	return base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(seed[:]))
}
