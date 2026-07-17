package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildTarGz creates a minimal tar.gz archive containing a single file with
// the given name and content. Returns the archive bytes.
func buildTarGz(t *testing.T, filename, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	body := []byte(content)
	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(body)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(body)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func buildEmptyTarGz(t *testing.T, name string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     name,
		Mode:     0755,
	}
	require.NoError(t, tw.WriteHeader(hdr))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// generateReleaseTestKey creates a synthetic ECDSA P-256 key pair for release
// signature tests and returns both the private key (for signing test
// fixtures) and its PinnedReleaseKey record (for the trust-anchor trial
// set). Production keys never appear in tests.
func generateReleaseTestKey(t *testing.T, expiresAt string) (*ecdsa.PrivateKey, PinnedReleaseKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return priv, PinnedReleaseKey{
		KeyID:        "test-" + expiresAt,
		ExpiresAt:    expiresAt,
		PublicKeyPEM: string(pemBytes),
	}
}

// signReleaseChecksums produces a detached ECDSA P-256 ASN.1/DER signature
// over data's SHA-256 digest, matching the producer's
// `openssl dgst -sha256 -sign` output format.
func signReleaseChecksums(t *testing.T, priv *ecdsa.PrivateKey, data []byte) []byte {
	t.Helper()
	digest := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	require.NoError(t, err)
	return sig
}
