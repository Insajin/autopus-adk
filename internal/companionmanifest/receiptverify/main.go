package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	receiptSchema       = "adk-companion-public-key-receipt.v1"
	receiptAlgorithm    = "ed25519"
	receiptKeyEncoding  = "base64-raw-32"
	maximumReceiptBytes = 16 * 1024
	maximumKeyBytes     = 4096
)

type receiptClaims struct {
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

type verifyOptions struct {
	receiptPath, signaturePath string
	signingKeyPath             string
	expectedKeyID              string
	expectedIssuedAt           string
	expectedExpiresAt          string
	expectedHandoff            string
	minimumRollbackFloor       uint64
	expectedPublicKeySHA256    string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "companion receipt verification:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	options := verifyOptions{}
	flags := flag.NewFlagSet("receiptverify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.receiptPath, "receipt", "", "Canonical receipt path")
	flags.StringVar(&options.signaturePath, "signature", "", "Raw detached signature path")
	flags.StringVar(&options.signingKeyPath, "signing-key", "", "Expected release signing key path")
	flags.StringVar(&options.expectedKeyID, "key-id", "", "Expected key identifier")
	flags.StringVar(&options.expectedIssuedAt, "issued-at", "", "Expected issuance time")
	flags.StringVar(&options.expectedExpiresAt, "expires-at", "", "Expected expiry time")
	flags.StringVar(&options.expectedHandoff, "handoff", "", "Expected handoff")
	flags.Uint64Var(&options.minimumRollbackFloor, "minimum-rollback-floor", 0, "Expected rollback floor")
	flags.StringVar(&options.expectedPublicKeySHA256, "public-key-sha256", "", "Optional pinned public key digest")
	if err := flags.Parse(arguments); err != nil {
		return errors.New("parse verification arguments")
	}
	if flags.NArg() != 0 {
		return errors.New("positional arguments are forbidden")
	}
	return verifyReceipt(options)
}

func verifyReceipt(options verifyOptions) error {
	if options.receiptPath == "" || options.signaturePath == "" ||
		options.signingKeyPath == "" || options.expectedKeyID == "" ||
		options.expectedIssuedAt == "" || options.expectedExpiresAt == "" ||
		options.expectedHandoff == "" || options.minimumRollbackFloor == 0 {
		return errors.New("required verification claim is missing")
	}
	receiptBytes, err := readRegularFile(options.receiptPath, maximumReceiptBytes)
	if err != nil {
		return errors.New("read canonical receipt")
	}
	signature, err := readRegularFile(options.signaturePath, ed25519.SignatureSize)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid detached signature bytes")
	}
	claims, publicKey, err := parseReceipt(receiptBytes)
	if err != nil {
		return err
	}
	if claims.KeyID != options.expectedKeyID || claims.IssuedAt != options.expectedIssuedAt ||
		claims.ExpiresAt != options.expectedExpiresAt || claims.Handoff != options.expectedHandoff ||
		claims.MinimumRollbackFloor != options.minimumRollbackFloor {
		return errors.New("receipt claims differ from release policy")
	}
	if options.expectedPublicKeySHA256 != "" &&
		subtle.ConstantTimeCompare([]byte(claims.PublicKeySHA256), []byte(options.expectedPublicKeySHA256)) != 1 {
		return errors.New("receipt public key differs from immutable pin")
	}
	privateKey, err := readPrivateKey(options.signingKeyPath)
	if err != nil {
		return err
	}
	defer clear(privateKey)
	expectedPublicKey := privateKey.Public().(ed25519.PublicKey)
	if subtle.ConstantTimeCompare(publicKey, expectedPublicKey) != 1 {
		return errors.New("receipt public key differs from release signing key")
	}
	if !ed25519.Verify(publicKey, receiptBytes, signature) {
		return errors.New("detached Ed25519 signature is invalid")
	}
	return nil
}

func parseReceipt(data []byte) (receiptClaims, ed25519.PublicKey, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var claims receiptClaims
	if err := decoder.Decode(&claims); err != nil {
		return receiptClaims{}, nil, errors.New("invalid receipt JSON")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return receiptClaims{}, nil, errors.New("receipt contains trailing data")
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !bytes.Equal(canonical, data) {
		return receiptClaims{}, nil, errors.New("receipt is not exact canonical JSON")
	}
	if claims.SchemaVersion != receiptSchema || claims.Algorithm != receiptAlgorithm ||
		claims.PublicKeyEncoding != receiptKeyEncoding {
		return receiptClaims{}, nil, errors.New("receipt key metadata is invalid")
	}
	publicKey, err := base64.StdEncoding.Strict().DecodeString(claims.PublicKeyBase64)
	if err != nil || len(publicKey) != ed25519.PublicKeySize ||
		base64.StdEncoding.EncodeToString(publicKey) != claims.PublicKeyBase64 {
		return receiptClaims{}, nil, errors.New("receipt public key encoding is invalid")
	}
	digest := sha256.Sum256(publicKey)
	wantDigest := "sha256:" + hex.EncodeToString(digest[:])
	if subtle.ConstantTimeCompare([]byte(claims.PublicKeySHA256), []byte(wantDigest)) != 1 {
		return receiptClaims{}, nil, errors.New("receipt public key digest is invalid")
	}
	issuedAt, issuedErr := canonicalTime(claims.IssuedAt)
	expiresAt, expiresErr := canonicalTime(claims.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || !expiresAt.After(issuedAt) {
		return receiptClaims{}, nil, errors.New("receipt validity window is invalid")
	}
	return claims, ed25519.PublicKey(publicKey), nil
}

func canonicalTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.UTC().Format(time.RFC3339) != value {
		return time.Time{}, errors.New("non-canonical UTC timestamp")
	}
	return parsed, nil
}

func readPrivateKey(path string) (ed25519.PrivateKey, error) {
	encoded, err := readRegularFile(path, maximumKeyBytes)
	if err != nil {
		return nil, errors.New("read release signing key")
	}
	return decodePrivateKeyAndClear(encoded)
}

func decodePrivateKeyAndClear(encoded []byte) (ed25519.PrivateKey, error) {
	defer clear(encoded)
	trimmed := bytes.TrimSpace(encoded)
	encoding := base64.StdEncoding.Strict()
	decoded := make([]byte, encoding.DecodedLen(len(trimmed)))
	decodedLength, err := encoding.Decode(decoded, trimmed)
	if err != nil || decodedLength != ed25519.PrivateKeySize {
		clear(decoded)
		return nil, errors.New("release signing key is invalid")
	}
	privateKey := ed25519.PrivateKey(decoded[:decodedLength])
	seed := privateKey.Seed()
	normalized := ed25519.NewKeyFromSeed(seed)
	clear(seed)
	if subtle.ConstantTimeCompare(privateKey, normalized) != 1 {
		clear(privateKey)
		clear(normalized)
		return nil, errors.New("release signing key is inconsistent")
	}
	clear(normalized)
	return privateKey, nil
}

func readRegularFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, errors.New("invalid regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("file identity changed")
	}
	return io.ReadAll(io.LimitReader(file, maximum+1))
}
