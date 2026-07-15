package cli

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

const cliReceiptVectorSeedHex = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"

type publicKeyReceiptCommandResult struct {
	stdout string
	stderr string
	err    error
}

// This RFC 8032 vector is local test material and is not release evidence.
func writePublicKeyReceiptTestKey(t *testing.T, dir string) (string, ed25519.PrivateKey) {
	t.Helper()
	seed, err := hex.DecodeString(cliReceiptVectorSeedHex)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	path := filepath.Join(dir, "signing-key")
	encoded := base64.StdEncoding.EncodeToString(privateKey) + "\n"
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, privateKey
}

func assertPublicKeyReceiptCommandNoLeak(
	t *testing.T,
	result publicKeyReceiptCommandResult,
	privateKey ed25519.PrivateKey,
	receiptPath string,
) {
	t.Helper()
	surface := []byte(result.stdout + "\n" + result.stderr)
	if result.err != nil {
		surface = append(surface, result.err.Error()...)
	}
	if receiptPath != "" {
		if receipt, err := os.ReadFile(receiptPath); err == nil {
			surface = append(surface, receipt...)
		}
	}
	seed := privateKey.Seed()
	forbidden := [][]byte{
		seed,
		privateKey,
		[]byte(hex.EncodeToString(seed)),
		[]byte(hex.EncodeToString(privateKey)),
		[]byte(base64.StdEncoding.EncodeToString(seed)),
		[]byte(base64.StdEncoding.EncodeToString(privateKey)),
	}
	for _, value := range forbidden {
		if bytes.Contains(surface, value) {
			t.Fatal("private signing material appeared in output or error")
		}
	}
}

func TestCompanionPublicKeyReceiptCommand_RegistersBundleOnlyContract(t *testing.T) {
	root := NewRootCmd()
	registered, _, err := root.Find([]string{"companion-manifest", "public-key-receipt"})
	if err != nil || registered.Name() != "public-key-receipt" {
		t.Fatalf("registered command = %v, error = %v", registered, err)
	}
	if registered.Flag("bundle-output") == nil {
		t.Fatal("bundle-output flag is missing")
	}
	for _, obsolete := range []string{"receipt-output", "signature-output"} {
		if registered.Flag(obsolete) != nil {
			t.Fatalf("obsolete non-atomic flag %q remains public", obsolete)
		}
	}
}

func TestCompanionPublicKeyReceiptCommand_LegacySiblingFlags_FailWithoutOutput(t *testing.T) {
	root := t.TempDir()
	keyPath, privateKey := writePublicKeyReceiptTestKey(t, root)
	receiptPath := filepath.Join(root, "receipt.json")
	signaturePath := filepath.Join(root, "receipt.sig")
	command := newCompanionPublicKeyReceiptCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{
		"--key-file", keyPath,
		"--receipt-output", receiptPath,
		"--signature-output", signaturePath,
	})
	err := command.Execute()
	result := publicKeyReceiptCommandResult{
		stdout: stdout.String(), stderr: stderr.String(), err: err,
	}
	if err == nil {
		t.Fatal("legacy sibling output contract was accepted")
	}
	for _, path := range []string{receiptPath, signaturePath} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("legacy contract created %s: %v", path, statErr)
		}
	}
	assertPublicKeyReceiptCommandNoLeak(t, result, privateKey, "")
}
