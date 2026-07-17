package selfupdate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActiveReleaseKeys_ExpiryBoundaryAndRotation(t *testing.T) {
	t.Parallel()

	_, expired := generateReleaseTestKey(t, "2026-07-16")
	_, boundary := generateReleaseTestKey(t, "2026-07-17")
	_, future := generateReleaseTestKey(t, "2099-12-31")

	active, err := activeReleaseKeys(
		[]pinnedReleaseKey{expired, boundary, future},
		time.Date(2026, 7, 17, 23, 59, 59, 0, time.UTC),
	)

	require.NoError(t, err)
	require.Len(t, active, 2)
	require.Contains(t, active, boundary.Fingerprint)
	require.Contains(t, active, future.Fingerprint)
}

func TestActiveReleaseKeys_RejectsMalformedTrustAnchors(t *testing.T) {
	t.Parallel()

	_, valid := generateReleaseTestKey(t, "2099-12-31")
	validBlock, _ := pem.Decode([]byte(valid.PublicKeyPEM))
	require.NotNil(t, validBlock)
	withHeaders := &pem.Block{
		Type:    validBlock.Type,
		Headers: map[string]string{"Proc-Type": "4,ENCRYPTED"},
		Bytes:   validBlock.Bytes,
	}
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	p384DER, err := x509.MarshalPKIXPublicKey(&p384.PublicKey)
	require.NoError(t, err)
	p384PEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: p384DER}))

	tests := []struct {
		name string
		key  pinnedReleaseKey
	}{
		{"missing set", pinnedReleaseKey{}},
		{"invalid expiry", withPinnedExpiry(valid, "2099-99-99")},
		{"invalid PEM", withPinnedPEM(valid, "not a pem")},
		{"leading PEM data", withPinnedPEM(valid, "\n"+valid.PublicKeyPEM)},
		{"PEM headers", withPinnedPEM(valid, string(pem.EncodeToMemory(withHeaders)))},
		{"trailing PEM data", withPinnedPEM(valid, valid.PublicKeyPEM+"garbage")},
		{"non P-256 key", withPinnedPEM(valid, p384PEM)},
		{"uppercase fingerprint", withPinnedFingerprint(valid, strings.ToUpper(valid.Fingerprint))},
		{"fingerprint mismatch", withPinnedFingerprint(valid, strings.Repeat("0", 64))},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := activeReleaseKeys([]pinnedReleaseKey{test.key}, referenceTime)

			require.ErrorIs(t, err, ErrMalformedEmbeddedReleaseKey)
			require.False(t, errors.Is(err, ErrAllReleaseSigningKeysExpired))
		})
	}

	_, err = activeReleaseKeys(nil, referenceTime)
	require.ErrorIs(t, err, ErrMalformedEmbeddedReleaseKey)
}

func TestActiveReleaseKeys_DuplicateFingerprintUsesMalformedSentinel(t *testing.T) {
	t.Parallel()

	_, valid := generateReleaseTestKey(t, "2099-12-31")

	_, err := activeReleaseKeys([]pinnedReleaseKey{valid, valid}, referenceTime)

	require.ErrorIs(t, err, ErrMalformedEmbeddedReleaseKey)
	require.False(t, errors.Is(err, ErrAllReleaseSigningKeysExpired))
}

func TestActiveReleaseKeys_AllExpiredUsesDedicatedSentinel(t *testing.T) {
	t.Parallel()

	_, first := generateReleaseTestKey(t, "2020-01-01")
	_, second := generateReleaseTestKey(t, "2026-07-16")

	_, err := activeReleaseKeys([]pinnedReleaseKey{first, second}, referenceTime)

	require.ErrorIs(t, err, ErrAllReleaseSigningKeysExpired)
	require.False(t, errors.Is(err, ErrMalformedEmbeddedReleaseKey))
}

func TestEmbeddedReleaseKeys_AreFullFingerprintP256AndActive(t *testing.T) {
	t.Parallel()

	want := []struct {
		fingerprint string
		expiresAt   string
	}{
		{"e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f", "2028-07-17"},
		{"93d9f681d829f2d0bdba7e1853e6acf9ae2ffd2c760355853218e920c35cc5ff", "2030-07-17"},
	}
	require.Len(t, embeddedReleaseKeys, len(want))
	for index, key := range embeddedReleaseKeys {
		publicKey, fingerprint, err := parsePinnedReleaseKey(key)
		require.NoError(t, err)
		require.Equal(t, "P-256", publicKey.Curve.Params().Name)
		require.Equal(t, want[index].fingerprint, fingerprint)
		require.Equal(t, want[index].expiresAt, key.ExpiresAt)
	}

	active, err := activeReleaseKeys(embeddedReleaseKeys[:], referenceTime)
	require.NoError(t, err)
	require.Len(t, active, len(want))
	for _, expected := range want {
		require.Contains(t, active, expected.fingerprint)
	}
}

func TestEmbeddedReleaseKeys_MatchCheckedInReleasePins(t *testing.T) {
	t.Parallel()

	require.Len(t, embeddedReleaseKeys, 2)
	for index, name := range []string{"k1", "k2"} {
		key := embeddedReleaseKeys[index]
		publicPEM, err := os.ReadFile(filepath.Join("..", "..", "scripts", "release-signing", "release-"+name+"-public.pem"))
		require.NoError(t, err)
		fingerprint, err := os.ReadFile(filepath.Join("..", "..", "scripts", "release-signing", "release-"+name+".fingerprint"))
		require.NoError(t, err)

		require.Equal(t, key.PublicKeyPEM+"\n", string(publicPEM))
		require.Equal(t, key.Fingerprint+"\n", string(fingerprint))
	}
}

func withPinnedExpiry(key pinnedReleaseKey, value string) pinnedReleaseKey {
	key.ExpiresAt = value
	return key
}

func withPinnedPEM(key pinnedReleaseKey, value string) pinnedReleaseKey {
	key.PublicKeyPEM = value
	return key
}

func withPinnedFingerprint(key pinnedReleaseKey, value string) pinnedReleaseKey {
	key.Fingerprint = value
	return key
}
