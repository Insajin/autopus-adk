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
	"regexp"
	"strings"
	"time"
)

const (
	manifestSchema   = "adk-companion-manifest.v1"
	receiptSchema    = "adk-companion-public-key-receipt.v1"
	receiptAlgorithm = "ed25519"
	receiptEncoding  = "base64-raw-32"
	maximumJSONBytes = 64 * 1024
)

var (
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	slugPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@/+\-]{0,255}$`)
)

type manifestClaims struct {
	SchemaVersion   string `json:"schema_version"`
	ArtifactDigest  string `json:"artifact_digest"`
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	Architecture    string `json:"architecture"`
	BuildProvenance string `json:"build_provenance"`
	Handoff         string `json:"handoff"`
	RollbackFloor   uint64 `json:"rollback_floor"`
	IssuedAt        string `json:"issued_at"`
	ExpiresAt       string `json:"expires_at"`
	KeyID           string `json:"key_id"`
}

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

type options struct {
	artifactPath, manifestPath, signaturePath string
	receiptPath, receiptSignaturePath         string
	signingKeyPath                            string
	keyID, version, platform, architecture    string
	handoff, publicKeySHA256, manifestSHA256  string
	minimumRollbackFloor                      uint64
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "companion manifest verification: verification failed")
		os.Exit(1)
	}
}

func run(arguments []string) error {
	var opts options
	flags := flag.NewFlagSet("manifestverify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.artifactPath, "artifact", "", "Artifact path")
	flags.StringVar(&opts.manifestPath, "manifest", "", "Canonical manifest path")
	flags.StringVar(&opts.signaturePath, "signature", "", "Raw manifest signature path")
	flags.StringVar(&opts.receiptPath, "receipt", "", "Canonical public key receipt path")
	flags.StringVar(&opts.receiptSignaturePath, "receipt-signature", "", "Raw receipt signature path")
	flags.StringVar(&opts.signingKeyPath, "signing-key", "", "Expected release signing key path")
	flags.StringVar(&opts.keyID, "key-id", "", "Expected key identifier")
	flags.StringVar(&opts.version, "version", "", "Expected release version")
	flags.StringVar(&opts.platform, "platform", "", "Expected platform")
	flags.StringVar(&opts.architecture, "architecture", "", "Expected architecture")
	flags.StringVar(&opts.handoff, "handoff", "", "Expected handoff")
	flags.Uint64Var(&opts.minimumRollbackFloor, "minimum-rollback-floor", 0, "Expected rollback floor")
	flags.StringVar(&opts.publicKeySHA256, "public-key-sha256", "", "Optional trusted public key digest")
	flags.StringVar(&opts.manifestSHA256, "manifest-sha256", "", "Optional trusted manifest digest")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("invalid arguments")
	}
	return verify(opts)
}

func verify(opts options) error {
	if opts.artifactPath == "" || opts.manifestPath == "" || opts.signaturePath == "" ||
		opts.receiptPath == "" || opts.receiptSignaturePath == "" || opts.keyID == "" ||
		opts.version == "" || opts.platform == "" || opts.architecture == "" ||
		opts.handoff == "" || opts.minimumRollbackFloor == 0 {
		return errors.New("missing verification input")
	}
	if opts.manifestSHA256 != "" && !digestPattern.MatchString(opts.manifestSHA256) {
		return errors.New("invalid trusted digest")
	}
	expectedPublicKeySHA256, err := trustedPublicKeySHA256(opts)
	if err != nil {
		return err
	}
	receiptBytes, err := readRegularFile(opts.receiptPath, maximumJSONBytes)
	if err != nil {
		return err
	}
	receiptSignature, err := readRegularFile(opts.receiptSignaturePath, ed25519.SignatureSize)
	if err != nil || len(receiptSignature) != ed25519.SignatureSize {
		return errors.New("invalid receipt signature")
	}
	receipt, publicKey, receiptIssued, receiptExpires, err := parseReceipt(receiptBytes)
	if err != nil {
		return err
	}
	if receipt.KeyID != opts.keyID || receipt.Handoff != opts.handoff ||
		receipt.MinimumRollbackFloor != opts.minimumRollbackFloor {
		return errors.New("receipt policy mismatch")
	}
	if subtle.ConstantTimeCompare([]byte(receipt.PublicKeySHA256),
		[]byte(expectedPublicKeySHA256)) != 1 {
		return errors.New("public key pin mismatch")
	}
	if !ed25519.Verify(publicKey, receiptBytes, receiptSignature) {
		return errors.New("invalid receipt signature")
	}

	manifestBytes, err := readRegularFile(opts.manifestPath, maximumJSONBytes)
	if err != nil {
		return err
	}
	if opts.manifestSHA256 != "" && !sameDigest(manifestBytes, opts.manifestSHA256) {
		return errors.New("manifest pin mismatch")
	}
	manifestSignature, err := readRegularFile(opts.signaturePath, ed25519.SignatureSize)
	if err != nil || len(manifestSignature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, manifestBytes, manifestSignature) {
		return errors.New("invalid manifest signature")
	}
	manifest, manifestIssued, manifestExpires, err := parseManifest(manifestBytes)
	if err != nil {
		return err
	}
	if manifest.KeyID != opts.keyID || manifest.Version != opts.version ||
		manifest.Platform != opts.platform || manifest.Architecture != opts.architecture ||
		manifest.Handoff != opts.handoff || manifest.RollbackFloor < receipt.MinimumRollbackFloor ||
		manifestIssued.Before(receiptIssued) || manifestExpires.After(receiptExpires) {
		return errors.New("manifest policy mismatch")
	}
	artifactDigest, err := hashRegularFile(opts.artifactPath)
	if err != nil || subtle.ConstantTimeCompare([]byte(manifest.ArtifactDigest), []byte(artifactDigest)) != 1 {
		return errors.New("artifact digest mismatch")
	}
	return nil
}

func trustedPublicKeySHA256(opts options) (string, error) {
	if (opts.publicKeySHA256 == "") == (opts.signingKeyPath == "") {
		return "", errors.New("exactly one public key trust anchor is required")
	}
	if opts.publicKeySHA256 != "" {
		if !digestPattern.MatchString(opts.publicKeySHA256) {
			return "", errors.New("invalid public key trust anchor")
		}
		return opts.publicKeySHA256, nil
	}
	return publicKeySHA256FromPrivateKey(opts.signingKeyPath)
}

func parseReceipt(data []byte) (receiptClaims, ed25519.PublicKey, time.Time, time.Time, error) {
	var claims receiptClaims
	if err := decodeCanonical(data, &claims); err != nil {
		return claims, nil, time.Time{}, time.Time{}, err
	}
	if claims.SchemaVersion != receiptSchema || claims.Algorithm != receiptAlgorithm ||
		claims.PublicKeyEncoding != receiptEncoding || !slugPattern.MatchString(claims.KeyID) ||
		!slugPattern.MatchString(claims.Handoff) || !digestPattern.MatchString(claims.PublicKeySHA256) {
		return claims, nil, time.Time{}, time.Time{}, errors.New("invalid receipt claims")
	}
	publicKey, err := base64.StdEncoding.Strict().DecodeString(claims.PublicKeyBase64)
	if err != nil || len(publicKey) != ed25519.PublicKeySize ||
		base64.StdEncoding.EncodeToString(publicKey) != claims.PublicKeyBase64 {
		return claims, nil, time.Time{}, time.Time{}, errors.New("invalid receipt key")
	}
	digest := sha256.Sum256(publicKey)
	if subtle.ConstantTimeCompare([]byte(claims.PublicKeySHA256),
		[]byte("sha256:"+hex.EncodeToString(digest[:]))) != 1 {
		return claims, nil, time.Time{}, time.Time{}, errors.New("invalid receipt key digest")
	}
	issued, expires, err := validityWindow(claims.IssuedAt, claims.ExpiresAt)
	return claims, ed25519.PublicKey(publicKey), issued, expires, err
}

func parseManifest(data []byte) (manifestClaims, time.Time, time.Time, error) {
	var claims manifestClaims
	if err := decodeCanonical(data, &claims); err != nil {
		return claims, time.Time{}, time.Time{}, err
	}
	if claims.SchemaVersion != manifestSchema || !digestPattern.MatchString(claims.ArtifactDigest) ||
		!slugPattern.MatchString(claims.Version) || !slugPattern.MatchString(claims.Platform) ||
		!slugPattern.MatchString(claims.Architecture) || !slugPattern.MatchString(claims.Handoff) ||
		!slugPattern.MatchString(claims.KeyID) || !validProvenance(claims.BuildProvenance) {
		return claims, time.Time{}, time.Time{}, errors.New("invalid manifest claims")
	}
	issued, expires, err := validityWindow(claims.IssuedAt, claims.ExpiresAt)
	return claims, issued, expires, err
}

func decodeCanonical(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return errors.New("invalid canonical JSON")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	canonical, err := json.Marshal(destination)
	if err != nil || !bytes.Equal(canonical, data) {
		return errors.New("non-canonical JSON")
	}
	return nil
}

func validityWindow(issuedValue, expiresValue string) (time.Time, time.Time, error) {
	issued, issuedErr := time.Parse(time.RFC3339, issuedValue)
	expires, expiresErr := time.Parse(time.RFC3339, expiresValue)
	if issuedErr != nil || expiresErr != nil || issued.UTC().Format(time.RFC3339) != issuedValue ||
		expires.UTC().Format(time.RFC3339) != expiresValue || !expires.After(issued) {
		return time.Time{}, time.Time{}, errors.New("invalid validity window")
	}
	return issued, expires, nil
}

func validProvenance(value string) bool {
	if value == "" || len(value) > 512 {
		return false
	}
	for index, char := range []byte(value) {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || (index > 0 && strings.ContainsRune("._:@/+-", rune(char)))) {
			return false
		}
	}
	return true
}
