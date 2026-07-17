package selfupdate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActiveKeys_ExcludesExpired verifies the pre-trial expiry gate: a key
// whose ExpiresAt has passed as of now is excluded from the active set.
func TestActiveKeys_ExcludesExpired(t *testing.T) {
	t.Parallel()

	_, expired := generateReleaseTestKey(t, "2020-01-01")
	_, active := generateReleaseTestKey(t, "2099-12-31")

	got := ActiveKeys([]PinnedReleaseKey{expired, active}, referenceTime)

	require.Len(t, got, 1)
	assert.True(t, got[0].Equal(mustParsePublicKey(t, active.PublicKeyPEM)))
}

// TestActiveKeys_BoundaryDateStillActive verifies a key is NOT excluded on
// its exact ExpiresAt date -- only strictly-after excludes it, matching
// install.sh's `[ "$now" \> "$EXPIRES_n" ]` lexicographic gate.
func TestActiveKeys_BoundaryDateStillActive(t *testing.T) {
	t.Parallel()

	_, pinned := generateReleaseTestKey(t, "2026-07-17")
	now := time.Date(2026, 7, 17, 23, 59, 59, 0, time.UTC)

	got := ActiveKeys([]PinnedReleaseKey{pinned}, now)

	assert.Len(t, got, 1)
}

// TestActiveKeys_DayAfterExpiryExcluded verifies the day immediately after
// ExpiresAt excludes the key.
func TestActiveKeys_DayAfterExpiryExcluded(t *testing.T) {
	t.Parallel()

	_, pinned := generateReleaseTestKey(t, "2026-07-17")
	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)

	got := ActiveKeys([]PinnedReleaseKey{pinned}, now)

	assert.Empty(t, got)
}

// TestActiveKeys_MalformedPEMExcludedNotPanicking verifies a malformed
// embedded PEM entry is excluded defensively instead of panicking, so one
// bad rotation entry cannot lock every client out of an otherwise-valid key.
func TestActiveKeys_MalformedPEMExcludedNotPanicking(t *testing.T) {
	t.Parallel()

	bad := PinnedReleaseKey{KeyID: "bad", ExpiresAt: "2099-12-31", PublicKeyPEM: "not a pem"}
	_, good := generateReleaseTestKey(t, "2099-12-31")

	got := ActiveKeys([]PinnedReleaseKey{bad, good}, referenceTime)

	require.Len(t, got, 1)
}

// TestActiveKeys_MalformedExpiresAtExcluded verifies a key with an
// unparsable ExpiresAt is excluded rather than crashing the filter.
func TestActiveKeys_MalformedExpiresAtExcluded(t *testing.T) {
	t.Parallel()

	_, badDate := generateReleaseTestKey(t, "not-a-date")

	got := ActiveKeys([]PinnedReleaseKey{badDate}, referenceTime)

	assert.Empty(t, got)
}

// TestActiveKeys_RejectsNonP256Curve verifies a key on a different curve
// (e.g. P-384) is excluded, since verification assumes P-256.
func TestActiveKeys_RejectsNonP256Curve(t *testing.T) {
	t.Parallel()

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	nonP256 := PinnedReleaseKey{KeyID: "p384", ExpiresAt: "2099-12-31", PublicKeyPEM: string(pemBytes)}

	got := ActiveKeys([]PinnedReleaseKey{nonP256}, referenceTime)

	assert.Empty(t, got)
}

// TestEmbeddedReleaseKeys_ProductionKeyIsValid is a regression guard on the
// production trust anchor: it must parse as a P-256 SPKI key and its KeyID
// bookkeeping constant must match sha256(SPKI DER)'s first 8 hex chars, so a
// future hand-edit of the embedded PEM cannot silently desync from its KeyID
// label.
func TestEmbeddedReleaseKeys_ProductionKeyIsValid(t *testing.T) {
	t.Parallel()

	require.Len(t, EmbeddedReleaseKeys, 1)
	key := EmbeddedReleaseKeys[0]

	pub := mustParsePublicKey(t, key.PublicKeyPEM)
	assert.Equal(t, "P-256", pub.Curve.Params().Name)

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	digest := sha256.Sum256(pubDER)
	assert.Equal(t, key.KeyID, hex.EncodeToString(digest[:])[:8])

	// Must still be active well before its documented expiry.
	active := ActiveKeys(EmbeddedReleaseKeys, referenceTime)
	assert.Len(t, active, 1)
}

func mustParsePublicKey(t *testing.T, pemStr string) *ecdsa.PublicKey {
	t.Helper()
	block, _ := pem.Decode([]byte(pemStr))
	require.NotNil(t, block)
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err)
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	require.True(t, ok)
	return ecdsaPub
}
