// Package companionmanifest defines the signed ADK companion release contract.
package companionmanifest

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	SchemaVersion    = "adk-companion-manifest.v1"
	maxManifestBytes = 64 * 1024
)

var (
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	slugPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@/+\-]{0,255}$`)
)

// Manifest is the exact v1 signed payload. Field order defines canonical JSON order.
type Manifest struct {
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

// CanonicalBytes validates and serializes a manifest without insignificant bytes.
func CanonicalBytes(manifest Manifest) ([]byte, error) {
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, errors.New("encode companion manifest")
	}
	return encoded, nil
}

// ParseStrict accepts only the exact, canonical v1 wire representation.
func ParseStrict(data []byte) (Manifest, error) {
	if len(data) == 0 || len(data) > maxManifestBytes {
		return Manifest{}, errors.New("invalid companion manifest size")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, errors.New("invalid companion manifest JSON")
	}
	if decoder.More() {
		return Manifest{}, errors.New("trailing companion manifest value")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return Manifest{}, errors.New("trailing companion manifest value")
	}
	canonical, err := CanonicalBytes(manifest)
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Manifest{}, errors.New("non-canonical companion manifest")
	}
	return manifest, nil
}

// SignCanonical returns the canonical manifest and its detached Ed25519 signature.
func SignCanonical(manifest Manifest, privateKey ed25519.PrivateKey) ([]byte, []byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("invalid signing key")
	}
	canonical, err := CanonicalBytes(manifest)
	if err != nil {
		return nil, nil, err
	}
	return canonical, ed25519.Sign(privateKey, canonical), nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return errors.New("unsupported companion manifest schema")
	}
	if !digestPattern.MatchString(manifest.ArtifactDigest) {
		return errors.New("invalid artifact digest")
	}
	fields := []struct{ name, value string }{
		{name: "version", value: manifest.Version},
		{name: "platform", value: manifest.Platform},
		{name: "architecture", value: manifest.Architecture},
		{name: "handoff", value: manifest.Handoff},
		{name: "key_id", value: manifest.KeyID},
	}
	for _, field := range fields {
		if !slugPattern.MatchString(field.value) {
			return fmt.Errorf("invalid %s", field.name)
		}
	}
	if !safeProvenance(manifest.BuildProvenance) {
		return errors.New("invalid build provenance")
	}
	issuedAt, err := parseCanonicalTime(manifest.IssuedAt)
	if err != nil {
		return errors.New("invalid issued_at")
	}
	expiresAt, err := parseCanonicalTime(manifest.ExpiresAt)
	if err != nil || !expiresAt.After(issuedAt) {
		return errors.New("invalid expires_at")
	}
	return nil
}

func safeProvenance(value string) bool {
	if value == "" || len(value) > 512 {
		return false
	}
	for index, char := range []byte(value) {
		if !((char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			(index > 0 && bytes.ContainsRune([]byte("._:@/+-"), rune(char)))) {
			return false
		}
	}
	return true
}

func parseCanonicalTime(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, errors.New("timestamp must be UTC")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.Format(time.RFC3339) != value {
		return time.Time{}, errors.New("non-canonical timestamp")
	}
	return parsed, nil
}
