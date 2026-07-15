package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const testSeedHex = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"

func TestVerifyReceipt_ValidDetachedSignatureAndClaims_Passes(t *testing.T) {
	options, _, _ := receiptFixture(t)
	if err := verifyReceipt(options); err != nil {
		t.Fatalf("verifyReceipt() error = %v", err)
	}
}

func TestVerifyReceipt_InvalidOrRepeatedSignature_FailsClosed(t *testing.T) {
	options, signature, _ := receiptFixture(t)
	cases := map[string][]byte{
		"invalid":  append([]byte(nil), signature...),
		"repeated": append(append([]byte(nil), signature...), signature...),
	}
	cases["invalid"][0] ^= 1
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(options.signaturePath, candidate, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := verifyReceipt(options); err == nil {
				t.Fatal("invalid detached signature was accepted")
			}
		})
	}
}

func TestVerifyReceipt_ClaimOrSigningKeyDrift_FailsClosed(t *testing.T) {
	options, _, receipt := receiptFixture(t)
	options.expectedKeyID = "different-key"
	if err := verifyReceipt(options); err == nil {
		t.Fatal("unexpected key ID was accepted")
	}
	options.expectedKeyID = "release-key"
	other := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	if err := os.WriteFile(options.signingKeyPath,
		[]byte(base64.StdEncoding.EncodeToString(other)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyReceipt(options); err == nil {
		t.Fatal("receipt signed by a different release key was accepted")
	}
	if err := os.WriteFile(options.signingKeyPath,
		[]byte(base64.StdEncoding.EncodeToString(testPrivateKey(t))), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(options.receiptPath, append(receipt, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyReceipt(options); err == nil {
		t.Fatal("non-canonical receipt bytes were accepted")
	}
}

func TestRun_ParsesExactPolicyAndRejectsPositionalArguments(t *testing.T) {
	options, _, _ := receiptFixture(t)
	arguments := verifyArguments(options)
	if err := run(arguments); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if err := run(append(arguments, "unexpected")); err == nil {
		t.Fatal("positional argument was accepted")
	}
	if err := run([]string{"--receipt"}); err == nil {
		t.Fatal("malformed flags were accepted")
	}
}

func TestVerifyReceipt_MalformedInputsFailClosed(t *testing.T) {
	base, signature, receipt := receiptFixture(t)
	cases := []struct {
		name   string
		mutate func(*verifyOptions) []byte
	}{
		{name: "missing option", mutate: func(options *verifyOptions) []byte {
			options.expectedHandoff = ""
			return receipt
		}},
		{name: "unknown field", mutate: func(_ *verifyOptions) []byte {
			return bytes.Replace(receipt, []byte(`"key_id"`), []byte(`"unknown"`), 1)
		}},
		{name: "key metadata", mutate: func(_ *verifyOptions) []byte {
			return bytes.Replace(receipt, []byte(`"ed25519"`), []byte(`"invalid"`), 1)
		}},
		{name: "public key encoding", mutate: func(_ *verifyOptions) []byte {
			return bytes.Replace(receipt, []byte(`"public_key_base64":"`), []byte(`"public_key_base64":"!`), 1)
		}},
		{name: "public key digest", mutate: func(_ *verifyOptions) []byte {
			return bytes.Replace(receipt, []byte(`"public_key_sha256":"sha256:`), []byte(`"public_key_sha256":"sha256:0`), 1)
		}},
		{name: "validity window", mutate: func(_ *verifyOptions) []byte {
			return bytes.Replace(receipt, []byte("2027-07-15T00:00:00Z"), []byte("2025-07-15T00:00:00Z"), 1)
		}},
		{name: "public key pin", mutate: func(options *verifyOptions) []byte {
			options.expectedPublicKeySHA256 = "sha256:" + string(bytes.Repeat([]byte{'0'}, 64))
			return receipt
		}},
		{name: "invalid key", mutate: func(options *verifyOptions) []byte {
			if err := os.WriteFile(options.signingKeyPath, []byte("invalid"), 0o600); err != nil {
				t.Fatal(err)
			}
			return receipt
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options := base
			candidate := test.mutate(&options)
			if err := os.WriteFile(options.receiptPath, candidate, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(options.signaturePath, signature, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := verifyReceipt(options); err == nil {
				t.Fatal("malformed receipt input was accepted")
			}
		})
	}
}

func receiptFixture(t *testing.T) (verifyOptions, []byte, []byte) {
	t.Helper()
	privateKey := testPrivateKey(t)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(publicKey)
	receipt := []byte(`{"schema_version":"adk-companion-public-key-receipt.v1","key_id":"release-key","algorithm":"ed25519","public_key_encoding":"base64-raw-32","public_key_base64":"` +
		base64.StdEncoding.EncodeToString(publicKey) + `","public_key_sha256":"sha256:` +
		hex.EncodeToString(digest[:]) + `","issued_at":"2026-07-14T00:00:00Z","expires_at":"2027-07-15T00:00:00Z","handoff":"v1","minimum_rollback_floor":5069}`)
	signature := ed25519.Sign(privateKey, receipt)
	dir := t.TempDir()
	receiptPath := filepath.Join(dir, "public-key-receipt.json")
	signaturePath := filepath.Join(dir, "public-key-receipt.sig")
	keyPath := filepath.Join(dir, "release-key")
	for path, data := range map[string][]byte{
		receiptPath: receipt, signaturePath: signature,
		keyPath: []byte(base64.StdEncoding.EncodeToString(privateKey)),
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return verifyOptions{
		receiptPath: receiptPath, signaturePath: signaturePath,
		signingKeyPath: keyPath, expectedKeyID: "release-key",
		expectedIssuedAt:  "2026-07-14T00:00:00Z",
		expectedExpiresAt: "2027-07-15T00:00:00Z",
		expectedHandoff:   "v1", minimumRollbackFloor: 5069,
		expectedPublicKeySHA256: "sha256:" + hex.EncodeToString(digest[:]),
	}, signature, receipt
}

func testPrivateKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	seed, err := hex.DecodeString(testSeedHex)
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func verifyArguments(options verifyOptions) []string {
	return []string{
		"--receipt", options.receiptPath,
		"--signature", options.signaturePath,
		"--signing-key", options.signingKeyPath,
		"--key-id", options.expectedKeyID,
		"--issued-at", options.expectedIssuedAt,
		"--expires-at", options.expectedExpiresAt,
		"--handoff", options.expectedHandoff,
		"--minimum-rollback-floor", strconv.FormatUint(options.minimumRollbackFloor, 10),
		"--public-key-sha256", options.expectedPublicKeySHA256,
	}
}
