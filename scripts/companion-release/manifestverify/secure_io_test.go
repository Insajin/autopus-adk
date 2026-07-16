package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicKeySHA256FromPrivateKey(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "signing-key")
	writeKey := func(value []byte) {
		t.Helper()
		encoded := base64.StdEncoding.EncodeToString(value)
		if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeKey(privateKey)
	digest := sha256.Sum256(publicKey)
	want := "sha256:" + hex.EncodeToString(digest[:])
	got, err := publicKeySHA256FromPrivateKey(path)
	if err != nil || got != want {
		t.Fatalf("public key digest = %q, %v; want %q", got, err, want)
	}

	inconsistent := append([]byte(nil), privateKey...)
	inconsistent[len(inconsistent)-1] ^= 1
	writeKey(inconsistent)
	clear(inconsistent)
	if _, err := publicKeySHA256FromPrivateKey(path); err == nil {
		t.Fatal("inconsistent private/public signing key passed")
	}
}
