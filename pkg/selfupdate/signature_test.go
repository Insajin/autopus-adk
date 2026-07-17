package selfupdate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// referenceTime is a fixed instant used across signature/pinnedkey tests so
// ExpiresAt fixtures ("2020-01-01", "2099-12-31") are unambiguous regardless
// of when the test suite actually runs.
var referenceTime = time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

// TestVerifyReleaseSignature_S1_NormalPath verifies that a signature made by
// the sole embedded key over the exact checksums bytes verifies successfully.
func TestVerifyReleaseSignature_S1_NormalPath(t *testing.T) {
	t.Parallel()

	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("abc123  autopus-adk_0.7.0_darwin_arm64.tar.gz\n")
	sig := signReleaseChecksums(t, priv, checksums)

	err := VerifyReleaseSignature(checksums, sig, []PinnedReleaseKey{pinned}, referenceTime)

	require.NoError(t, err)
}

// TestVerifyReleaseSignature_S2_TamperedChecksums verifies that a signature
// valid for the original checksums bytes fails once a single byte changes.
func TestVerifyReleaseSignature_S2_TamperedChecksums(t *testing.T) {
	t.Parallel()

	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("abc123  autopus-adk_0.7.0_darwin_arm64.tar.gz\n")
	sig := signReleaseChecksums(t, priv, checksums)
	tampered := []byte("abc124  autopus-adk_0.7.0_darwin_arm64.tar.gz\n")

	err := VerifyReleaseSignature(tampered, sig, []PinnedReleaseKey{pinned}, referenceTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no trusted release signing key verified")
}

// TestVerifyReleaseSignature_S4S6_AttackerKeyOutsideEmbeddedSet verifies that
// a signature made by a key outside the embedded set is rejected even though
// the signature is cryptographically valid for its own (untrusted) key. This
// also covers S6's same-origin full-replacement threat: an attacker who
// rewrites archive+checksums+sig together still signs with a key the client
// never embedded.
func TestVerifyReleaseSignature_S4S6_AttackerKeyOutsideEmbeddedSet(t *testing.T) {
	t.Parallel()

	attackerPriv, _ := generateReleaseTestKey(t, "2099-12-31")
	_, embeddedPinned := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("abc123  autopus-adk_0.7.0_darwin_arm64.tar.gz\n")
	sig := signReleaseChecksums(t, attackerPriv, checksums)

	err := VerifyReleaseSignature(checksums, sig, []PinnedReleaseKey{embeddedPinned}, referenceTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no trusted release signing key verified")
}

// TestVerifyReleaseSignature_S8b_SoleKeyExpired verifies the pre-trial expiry
// gate excludes the only embedded key before any cryptographic attempt, even
// though the signature itself is valid for that key.
func TestVerifyReleaseSignature_S8b_SoleKeyExpired(t *testing.T) {
	t.Parallel()

	priv, pinned := generateReleaseTestKey(t, "2020-01-01")
	checksums := []byte("checksum fixture\n")
	sig := signReleaseChecksums(t, priv, checksums)

	err := VerifyReleaseSignature(checksums, sig, []PinnedReleaseKey{pinned}, referenceTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all embedded keys expired")
}

// TestVerifyReleaseSignature_S8c_RotationWindow verifies that a 2-key
// embedded set (K1, newly rotated-in K2) still verifies a signature made
// with K2 -- the rotation window multi-trial passes.
func TestVerifyReleaseSignature_S8c_RotationWindow(t *testing.T) {
	t.Parallel()

	_, pinned1 := generateReleaseTestKey(t, "2099-12-31")
	priv2, pinned2 := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("checksum fixture\n")
	sig := signReleaseChecksums(t, priv2, checksums)

	err := VerifyReleaseSignature(checksums, sig, []PinnedReleaseKey{pinned1, pinned2}, referenceTime)

	require.NoError(t, err)
}

// TestVerifyReleaseSignature_EmptyKeySet verifies that an empty pinned key
// set is treated as "all expired" rather than panicking or passing.
func TestVerifyReleaseSignature_EmptyKeySet(t *testing.T) {
	t.Parallel()

	err := VerifyReleaseSignature([]byte("checksum fixture\n"), []byte("not-a-signature"), nil, referenceTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all embedded keys expired")
}

// --- R3: Go <-> openssl ECDSA-over-SHA256 interoperability ---

func requireOpenSSL(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not found in PATH; skipping interop test")
	}
}

// TestVerifyReleaseSignature_InteropGoSignsOpenSSLVerifies signs with Go's
// ecdsa.SignASN1 and verifies with the openssl CLI, confirming Go's stdlib
// output matches what the producer's `openssl dgst -sha256 -verify` step
// (and install.sh's verify_signature) expect.
func TestVerifyReleaseSignature_InteropGoSignsOpenSSLVerifies(t *testing.T) {
	requireOpenSSL(t)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	checksums := []byte("interop fixture: go signs, openssl verifies\n")
	digest := sha256.Sum256(checksums)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	require.NoError(t, err)

	dir := t.TempDir()
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPath := filepath.Join(dir, "pub.pem")
	require.NoError(t, os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600))
	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, checksums, 0o600))
	sigPath := filepath.Join(dir, "checksums.txt.sig")
	require.NoError(t, os.WriteFile(sigPath, sig, 0o600))

	out, err := exec.Command("openssl", "dgst", "-sha256", "-verify", pubPath, "-signature", sigPath, checksumsPath).CombinedOutput()
	require.NoError(t, err, "openssl verify failed: %s", out)
	assert.Contains(t, string(out), "Verified OK")
}

// TestVerifyReleaseSignature_InteropOpenSSLSignsGoVerifies signs with the
// openssl CLI (matching the producer's actual `openssl dgst -sha256 -sign`
// step) and verifies with VerifyReleaseSignature, confirming the Go consumer
// accepts real producer-shaped signatures.
func TestVerifyReleaseSignature_InteropOpenSSLSignsGoVerifies(t *testing.T) {
	requireOpenSSL(t)

	dir := t.TempDir()
	privPath := filepath.Join(dir, "priv.pem")
	pubPath := filepath.Join(dir, "pub.pem")
	require.NoError(t, exec.Command("openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", privPath).Run())
	require.NoError(t, exec.Command("openssl", "ec", "-in", privPath, "-pubout", "-out", pubPath).Run())

	checksums := []byte("interop fixture: openssl signs, go verifies\n")
	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, checksums, 0o600))
	sigPath := filepath.Join(dir, "checksums.txt.sig")
	signOut, err := exec.Command("openssl", "dgst", "-sha256", "-sign", privPath, "-out", sigPath, checksumsPath).CombinedOutput()
	require.NoError(t, err, "openssl sign failed: %s", signOut)

	sig, err := os.ReadFile(sigPath)
	require.NoError(t, err)
	pubPEM, err := os.ReadFile(pubPath)
	require.NoError(t, err)

	pinned := PinnedReleaseKey{KeyID: "interop", ExpiresAt: "2099-12-31", PublicKeyPEM: string(pubPEM)}
	verifyErr := VerifyReleaseSignature(checksums, sig, []PinnedReleaseKey{pinned}, referenceTime)

	require.NoError(t, verifyErr)
}
