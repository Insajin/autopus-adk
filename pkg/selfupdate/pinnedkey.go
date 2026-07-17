package selfupdate

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"
)

const releaseKeyExpiryLayout = "2006-01-02"

type pinnedReleaseKey struct {
	Fingerprint  string
	ExpiresAt    string
	PublicKeyPEM string
}

// K1 is the active release signer. K2 is prepositioned as the offline-next
// rotation anchor; the v0.50.73 workflow signs with K1 only.
var embeddedReleaseKeys = [...]pinnedReleaseKey{
	{
		Fingerprint: "e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f",
		ExpiresAt:   "2028-07-17",
		PublicKeyPEM: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDFjY80Lc2GJSsd8M6uAO/v7AZK3Z
1sPEXrK4Hbm4m4+ykavvcoKlpZ5sn/T/l2InDXuhxkdX6aFv57bicik2Ug==
-----END PUBLIC KEY-----`,
	},
	{
		Fingerprint: "93d9f681d829f2d0bdba7e1853e6acf9ae2ffd2c760355853218e920c35cc5ff",
		ExpiresAt:   "2030-07-17",
		PublicKeyPEM: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEp+d1byDqWFismSIMWhTEHnbo/pdp
7JVZwhXOIZJb0q2WHLxwMD7P77Fkr75Xnx1qYZgfvIl9Sg8Z+V9gSaq8Og==
-----END PUBLIC KEY-----`,
	},
}

func activeReleaseKeys(keys []pinnedReleaseKey, now time.Time) (map[string]*ecdsa.PublicKey, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no keys configured", ErrMalformedEmbeddedReleaseKey)
	}

	active := make(map[string]*ecdsa.PublicKey, len(keys))
	seen := make(map[string]struct{}, len(keys))
	nowDate := now.UTC().Format(releaseKeyExpiryLayout)
	for index, key := range keys {
		publicKey, fingerprint, err := parsePinnedReleaseKey(key)
		if err != nil {
			return nil, fmt.Errorf("%w at index %d: %v", ErrMalformedEmbeddedReleaseKey, index, err)
		}
		if _, duplicate := seen[fingerprint]; duplicate {
			return nil, fmt.Errorf("%w at index %d: duplicate fingerprint", ErrMalformedEmbeddedReleaseKey, index)
		}
		seen[fingerprint] = struct{}{}

		expiresAt, err := time.Parse(releaseKeyExpiryLayout, key.ExpiresAt)
		if err != nil || expiresAt.Format(releaseKeyExpiryLayout) != key.ExpiresAt {
			return nil, fmt.Errorf("%w at index %d: invalid expiry date", ErrMalformedEmbeddedReleaseKey, index)
		}
		if nowDate <= key.ExpiresAt {
			active[fingerprint] = publicKey
		}
	}
	if len(active) == 0 {
		return nil, ErrAllReleaseSigningKeysExpired
	}
	return active, nil
}

func parsePinnedReleaseKey(key pinnedReleaseKey) (*ecdsa.PublicKey, string, error) {
	if !validReleaseKeyFingerprint(key.Fingerprint) {
		return nil, "", fmt.Errorf("invalid fingerprint")
	}
	pemBytes := []byte(key.PublicKeyPEM)
	if !bytes.HasPrefix(pemBytes, []byte("-----BEGIN PUBLIC KEY-----")) {
		return nil, "", fmt.Errorf("PUBLIC KEY PEM block must begin at byte zero")
	}
	block, rest := pem.Decode(pemBytes)
	if block == nil || block.Type != "PUBLIC KEY" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return nil, "", fmt.Errorf("expected exactly one PUBLIC KEY PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("parse SPKI: %w", err)
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return nil, "", fmt.Errorf("expected ECDSA P-256 public key")
	}
	digest := sha256.Sum256(block.Bytes)
	fingerprint := hex.EncodeToString(digest[:])
	if fingerprint != key.Fingerprint {
		return nil, "", fmt.Errorf("SPKI fingerprint mismatch")
	}
	return publicKey, fingerprint, nil
}

func validReleaseKeyFingerprint(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
