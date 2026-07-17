package selfupdate

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var referenceTime = time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

func TestVerifyReleaseSignatures_AcceptsKnownSignatureAmongUnknownEntries(t *testing.T) {
	t.Parallel()

	knownPrivate, known := generateReleaseTestKey(t, "2099-12-31")
	unknownPrivate, unknown := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("abc123  autopus-adk_0.50.73_darwin_arm64.tar.gz\n")
	envelope := releaseSignatureEnvelope(t, checksums,
		testEnvelopeSigner{private: unknownPrivate, fingerprint: unknown.Fingerprint},
		testEnvelopeSigner{private: knownPrivate, fingerprint: known.Fingerprint},
	)

	err := verifyReleaseSignatures(checksums, envelope, []pinnedReleaseKey{known}, referenceTime)

	require.NoError(t, err)
}

func TestVerifyReleaseSignatures_FailsClosedForTamperingAndUntrustedSigners(t *testing.T) {
	t.Parallel()

	trustedPrivate, trusted := generateReleaseTestKey(t, "2099-12-31")
	attackerPrivate, attacker := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("abc123  archive.tar.gz\n")

	tests := []struct {
		name      string
		payload   []byte
		signers   []testEnvelopeSigner
		wantError error
	}{
		{
			name:      "tampered checksums",
			payload:   []byte("abc124  archive.tar.gz\n"),
			signers:   []testEnvelopeSigner{{private: trustedPrivate, fingerprint: trusted.Fingerprint}},
			wantError: ErrNoTrustedReleaseSignature,
		},
		{
			name:      "attacker fingerprint and signature",
			payload:   checksums,
			signers:   []testEnvelopeSigner{{private: attackerPrivate, fingerprint: attacker.Fingerprint}},
			wantError: ErrNoTrustedReleaseSignature,
		},
		{
			name:      "trusted fingerprint with attacker signature",
			payload:   checksums,
			signers:   []testEnvelopeSigner{{private: attackerPrivate, fingerprint: trusted.Fingerprint}},
			wantError: ErrNoTrustedReleaseSignature,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			envelope := releaseSignatureEnvelope(t, checksums, test.signers...)

			err := verifyReleaseSignatures(test.payload, envelope, []pinnedReleaseKey{trusted}, referenceTime)

			require.ErrorIs(t, err, test.wantError)
		})
	}
}

func TestParseReleaseSignatureEnvelope_RejectsMalformedInput(t *testing.T) {
	t.Parallel()

	private, pinned := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("checksum fixture\n")
	valid := releaseSignatureEnvelope(t, checksums,
		testEnvelopeSigner{private: private, fingerprint: pinned.Fingerprint},
	)
	validLine := strings.Split(string(valid), "\n")[1]
	validEncoded := strings.Split(validLine, "\t")[1]
	validDER, err := base64.StdEncoding.DecodeString(validEncoded)
	require.NoError(t, err)
	uppercase := strings.ToUpper(pinned.Fingerprint) + validLine[64:]
	badDER, err := asn1.Marshal(struct{ R, S *big.Int }{big.NewInt(0), big.NewInt(1)})
	require.NoError(t, err)
	badDERLine := pinned.Fingerprint + "\t" + base64.StdEncoding.EncodeToString(badDER)
	smallDER, err := asn1.Marshal(struct{ R, S *big.Int }{big.NewInt(1), big.NewInt(1)})
	require.NoError(t, err)
	unpaddedBase64 := strings.TrimRight(base64.StdEncoding.EncodeToString(smallDER), "=")
	trailingDER := base64.StdEncoding.EncodeToString(append(validDER, 0))

	tests := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"wrong header", "AUTOPUS-RELEASE-SIGNATURE-V2\n" + validLine + "\n"},
		{"header only", ReleaseSignatureEnvelopeHeader + "\n"},
		{"UTF-8 BOM", "\ufeff" + string(valid)},
		{"CRLF", strings.ReplaceAll(string(valid), "\n", "\r\n")},
		{"NUL", ReleaseSignatureEnvelopeHeader + "\n" + validLine + "\x00\n"},
		{"missing final newline", strings.TrimSuffix(string(valid), "\n")},
		{"blank record", ReleaseSignatureEnvelopeHeader + "\n\n" + validLine + "\n"},
		{"uppercase fingerprint", ReleaseSignatureEnvelopeHeader + "\n" + uppercase + "\n"},
		{"short fingerprint", ReleaseSignatureEnvelopeHeader + "\n" + validLine[1:] + "\n"},
		{"space separator", ReleaseSignatureEnvelopeHeader + "\n" + strings.Replace(validLine, "\t", " ", 1) + "\n"},
		{"invalid base64", ReleaseSignatureEnvelopeHeader + "\n" + pinned.Fingerprint + "\t***\n"},
		{"unpadded base64", ReleaseSignatureEnvelopeHeader + "\n" + pinned.Fingerprint + "\t" + unpaddedBase64 + "\n"},
		{"invalid DER integers", ReleaseSignatureEnvelopeHeader + "\n" + badDERLine + "\n"},
		{"trailing DER data", ReleaseSignatureEnvelopeHeader + "\n" + pinned.Fingerprint + "\t" + trailingDER + "\n"},
		{"duplicate fingerprint", ReleaseSignatureEnvelopeHeader + "\n" + validLine + "\n" + validLine + "\n"},
		{"oversized record", ReleaseSignatureEnvelopeHeader + "\n" + strings.Repeat("a", MaxReleaseSignatureLineSize+1) + "\n"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseReleaseSignatureEnvelope([]byte(test.data))

			require.ErrorIs(t, err, ErrMalformedReleaseSignatureEnvelope)
		})
	}
}

func TestParseReleaseSignatureEnvelope_RejectsCountAndSizeOverflow(t *testing.T) {
	t.Parallel()

	private, _ := generateReleaseTestKey(t, "2099-12-31")
	checksums := []byte("checksum fixture\n")
	signers := make([]testEnvelopeSigner, 0, MaxReleaseSignatureCount+1)
	for range MaxReleaseSignatureCount + 1 {
		_, key := generateReleaseTestKey(t, "2099-12-31")
		signers = append(signers, testEnvelopeSigner{private: private, fingerprint: key.Fingerprint})
	}
	tooMany := releaseSignatureEnvelope(t, checksums, signers...)

	_, err := parseReleaseSignatureEnvelope(tooMany)
	require.ErrorIs(t, err, ErrMalformedReleaseSignatureEnvelope)

	oversized := append([]byte(ReleaseSignatureEnvelopeHeader+"\n"), make([]byte, MaxReleaseSignatureEnvelopeSize)...)
	_, err = parseReleaseSignatureEnvelope(oversized)
	require.ErrorIs(t, err, ErrMalformedReleaseSignatureEnvelope)
}

func TestVerifyReleaseSignatures_DistinguishesExpiredAndMalformedKeys(t *testing.T) {
	t.Parallel()

	private, expired := generateReleaseTestKey(t, "2020-01-01")
	checksums := []byte("checksum fixture\n")
	envelope := releaseSignatureEnvelope(t, checksums,
		testEnvelopeSigner{private: private, fingerprint: expired.Fingerprint},
	)

	err := verifyReleaseSignatures(checksums, envelope, []pinnedReleaseKey{expired}, referenceTime)
	require.ErrorIs(t, err, ErrAllReleaseSigningKeysExpired)
	require.False(t, errors.Is(err, ErrMalformedEmbeddedReleaseKey))

	malformed := expired
	malformed.ExpiresAt = "not-a-date"
	err = verifyReleaseSignatures(checksums, envelope, []pinnedReleaseKey{malformed}, referenceTime)
	require.ErrorIs(t, err, ErrMalformedEmbeddedReleaseKey)
	require.False(t, errors.Is(err, ErrAllReleaseSigningKeysExpired))
}

type testEnvelopeSigner struct {
	private     *ecdsa.PrivateKey
	fingerprint string
}

func releaseSignatureEnvelope(t *testing.T, checksums []byte, signers ...testEnvelopeSigner) []byte {
	t.Helper()
	var builder strings.Builder
	builder.WriteString(ReleaseSignatureEnvelopeHeader + "\n")
	for _, signer := range signers {
		digest := sha256.Sum256(checksums)
		signature, err := ecdsa.SignASN1(rand.Reader, signer.private, digest[:])
		require.NoError(t, err)
		fmt.Fprintf(&builder, "%s\t%s\n", signer.fingerprint, base64.StdEncoding.EncodeToString(signature))
	}
	return []byte(builder.String())
}
