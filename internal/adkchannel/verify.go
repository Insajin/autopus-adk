package adkchannel

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	MaxDocumentBytes  = 8192
	channelSchema     = "adk-channel.v1"
	channelName       = "stable"
	channelRepository = "Insajin/autopus-adk"
)

type Options struct {
	PublicKeyBase64 string
	ExpectedKeyID   string
	Now             func() time.Time
}

type SignedBytes struct {
	Document  []byte
	Signature []byte
}

type Document struct {
	SchemaVersion string `json:"schema_version"`
	Channel       string `json:"channel"`
	Repository    string `json:"repository"`
	Version       string `json:"version"`
	Tag           string `json:"tag"`
	IssuedAt      string `json:"issued_at"`
	ExpiresAt     string `json:"expires_at"`
	ChannelKeyID  string `json:"channel_key_id"`
}

type Verified struct {
	Document  Document
	version   version
	expiresAt time.Time
}

type version [3]uint64

func Verify(signed SignedBytes, options Options) (Verified, error) {
	return verify(signed, options, false)
}

func verify(signed SignedBytes, options Options, allowExpired bool) (Verified, error) {
	if len(signed.Document) == 0 || len(signed.Document) > MaxDocumentBytes {
		return Verified{}, fmt.Errorf("document size must be between 1 and %d bytes", MaxDocumentBytes)
	}
	if len(signed.Signature) != ed25519.SignatureSize {
		return Verified{}, fmt.Errorf("signature must be %d bytes", ed25519.SignatureSize)
	}
	if options.ExpectedKeyID == "" {
		return Verified{}, fmt.Errorf("expected key id is required")
	}
	if options.Now == nil {
		return Verified{}, fmt.Errorf("clock is required")
	}

	publicKeyBytes, err := decodeCanonicalBase64(options.PublicKeyBase64, ed25519.PublicKeySize)
	if err != nil || len(publicKeyBytes) != ed25519.PublicKeySize {
		return Verified{}, fmt.Errorf("public key must be canonical base64 for %d bytes", ed25519.PublicKeySize)
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), signed.Document, signed.Signature) {
		return Verified{}, fmt.Errorf("signature verification failed")
	}

	var document Document
	if err := json.Unmarshal(signed.Document, &document); err != nil {
		return Verified{}, fmt.Errorf("document JSON is invalid: %w", err)
	}
	if document.ChannelKeyID != options.ExpectedKeyID {
		return Verified{}, fmt.Errorf("channel key id does not match expected key")
	}
	if document.SchemaVersion != channelSchema {
		return Verified{}, fmt.Errorf("schema_version must be %q", channelSchema)
	}
	if document.Channel != channelName {
		return Verified{}, fmt.Errorf("channel must be %q", channelName)
	}
	if document.Repository != channelRepository {
		return Verified{}, fmt.Errorf("repository must be %q", channelRepository)
	}

	parsedVersion, err := parseVersion(document.Version)
	if err != nil {
		return Verified{}, fmt.Errorf("version is invalid: %w", err)
	}
	if document.Tag != "v"+document.Version {
		return Verified{}, fmt.Errorf("tag must be v<version>")
	}
	issuedAt, err := parseCanonicalUTC(document.IssuedAt)
	if err != nil {
		return Verified{}, fmt.Errorf("issued_at is invalid: %w", err)
	}
	expiresAt, err := parseCanonicalUTC(document.ExpiresAt)
	if err != nil {
		return Verified{}, fmt.Errorf("expires_at is invalid: %w", err)
	}
	if !issuedAt.Before(expiresAt) {
		return Verified{}, fmt.Errorf("issued_at must precede expires_at")
	}
	now := options.Now()
	if now.Before(issuedAt) || (!allowExpired && !now.Before(expiresAt)) {
		return Verified{}, fmt.Errorf("document is outside its validity window")
	}

	return Verified{Document: document, version: parsedVersion, expiresAt: expiresAt}, nil
}

func VerifyUpdate(candidate SignedBytes, current *SignedBytes, options Options) (Verified, error) {
	frozenOptions := options
	if options.Now != nil {
		now := options.Now()
		frozenOptions.Now = func() time.Time { return now }
	}
	verifiedCandidate, err := Verify(candidate, frozenOptions)
	if err != nil {
		return Verified{}, fmt.Errorf("candidate: %w", err)
	}
	if current == nil {
		return verifiedCandidate, nil
	}
	verifiedCurrent, err := verify(*current, frozenOptions, true)
	if err != nil {
		return Verified{}, fmt.Errorf("current: %w", err)
	}

	switch compareVersions(verifiedCandidate.version, verifiedCurrent.version) {
	case -1:
		return Verified{}, fmt.Errorf("candidate version is lower than current version")
	case 0:
		if !verifiedCandidate.expiresAt.After(verifiedCurrent.expiresAt) {
			return Verified{}, fmt.Errorf("equal version must move expiry later")
		}
	}
	return verifiedCandidate, nil
}

func VerifyFiles(documentPath, signaturePath string, options Options) (Verified, error) {
	signed, err := ReadSignedFiles(documentPath, signaturePath)
	if err != nil {
		return Verified{}, err
	}
	return Verify(signed, options)
}

func ReadSignedFiles(documentPath, signaturePath string) (SignedBytes, error) {
	document, err := readRegularFile(documentPath, MaxDocumentBytes)
	if err != nil {
		return SignedBytes{}, fmt.Errorf("read document: %w", err)
	}
	signature, err := readRegularFile(signaturePath, ed25519.SignatureSize)
	if err != nil {
		return SignedBytes{}, fmt.Errorf("read signature: %w", err)
	}
	return SignedBytes{Document: document, Signature: signature}, nil
}

func parseVersion(text string) (version, error) {
	parts := strings.Split(text, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("must contain exactly three numeric components")
	}
	var parsed version
	for index, part := range parts {
		if part == "" {
			return version{}, fmt.Errorf("component %d is empty", index+1)
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return version{}, fmt.Errorf("component %d is not numeric", index+1)
			}
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return version{}, fmt.Errorf("component %d overflows uint64", index+1)
		}
		parsed[index] = value
	}
	return parsed, nil
}

func compareVersions(left, right version) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func parseCanonicalUTC(text string) (time.Time, error) {
	if len(text) != len("2006-01-02T15:04:05Z") || !strings.HasSuffix(text, "Z") {
		return time.Time{}, fmt.Errorf("must use YYYY-MM-DDTHH:MM:SSZ")
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil || parsed.UTC().Format("2006-01-02T15:04:05Z") != text {
		return time.Time{}, fmt.Errorf("must be a canonical UTC instant")
	}
	return parsed.UTC(), nil
}

func readRegularFile(path string, maximum int) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > int64(maximum) {
		return nil, fmt.Errorf("must be a regular file between 1 and %d bytes", maximum)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, statErr := file.Stat()
	if statErr != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = file.Close()
		return nil, fmt.Errorf("file changed while opening")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(data) < 1 || len(data) > maximum {
		return nil, fmt.Errorf("file size changed while reading")
	}
	return data, nil
}

func decodeCanonicalBase64(text string, maximum int) ([]byte, error) {
	if text == "" || len(text) > base64.StdEncoding.EncodedLen(maximum) {
		return nil, fmt.Errorf("encoded value has invalid size")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(text)
	if err != nil || len(decoded) > maximum || base64.StdEncoding.EncodeToString(decoded) != text {
		return nil, fmt.Errorf("encoded value is not canonical base64")
	}
	return decoded, nil
}
