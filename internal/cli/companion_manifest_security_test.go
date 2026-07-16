package cli

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanionManifestSign_InconsistentFullLengthKey_LeavesOutputsAbsent(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "auto")
	manifest := filepath.Join(dir, "manifest.json")
	signature := filepath.Join(dir, "manifest.sig")
	if err := os.WriteFile(artifact, []byte("artifact-v0.50.69"), 0o700); err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	privateKey[len(privateKey)-1] ^= 1
	command := newCompanionManifestSignCmd()
	var output bytes.Buffer
	command.SetIn(strings.NewReader(base64.StdEncoding.EncodeToString(privateKey)))
	command.SetOut(&output)
	command.SetArgs(companionSignArgs(artifact, manifest, signature))
	if err := command.Execute(); err == nil {
		t.Fatal("inconsistent 64-byte private key was accepted")
	}
	if output.Len() != 0 {
		t.Fatalf("failed signer stdout = %q", output.String())
	}
	for _, path := range []string{manifest, signature} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed signer created %s: %v", filepath.Base(path), err)
		}
	}
}
