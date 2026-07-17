package selfupdate

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	// ReleaseSignatureEnvelopeHeader identifies the only accepted wire format.
	ReleaseSignatureEnvelopeHeader = "AUTOPUS-RELEASE-SIGNATURE-V1"
	// MaxReleaseSignatureEnvelopeSize bounds the complete untrusted asset.
	MaxReleaseSignatureEnvelopeSize = 4 << 10
	// MaxReleaseSignatureLineSize bounds one fingerprint/signature record.
	MaxReleaseSignatureLineSize = 256
	// MaxReleaseSignatureCount bounds cryptographic work and rotation overlap.
	MaxReleaseSignatureCount = 16
)

var (
	// ErrMalformedReleaseSignatureEnvelope identifies invalid V1 wire data.
	ErrMalformedReleaseSignatureEnvelope = errors.New("malformed release signature envelope")
	// ErrMalformedEmbeddedReleaseKey identifies an invalid compiled trust anchor.
	ErrMalformedEmbeddedReleaseKey = errors.New("malformed embedded release signing key")
	// ErrAllReleaseSigningKeysExpired identifies a valid but fully expired key set.
	ErrAllReleaseSigningKeysExpired = errors.New("all embedded keys expired")
	// ErrNoTrustedReleaseSignature means no known active key verified the payload.
	ErrNoTrustedReleaseSignature = errors.New("no trusted release signing key verified")
)

type releaseSignatureEntry struct {
	fingerprint string
	r           *big.Int
	s           *big.Int
}

type ecdsaASN1Signature struct {
	R *big.Int
	S *big.Int
}

// VerifyReleaseSignature verifies a V1 release-signature envelope against
// the trust anchors compiled into this binary.
func VerifyReleaseSignature(checksums, envelope []byte) error {
	return verifyReleaseSignatures(checksums, envelope, embeddedReleaseKeys[:], time.Now())
}

func verifyReleaseSignatures(
	checksums, envelope []byte,
	keys []pinnedReleaseKey,
	now time.Time,
) error {
	entries, err := parseReleaseSignatureEnvelope(envelope)
	if err != nil {
		return err
	}
	active, err := activeReleaseKeys(keys, now)
	if err != nil {
		return err
	}

	digest := sha256.Sum256(checksums)
	for _, entry := range entries {
		publicKey, known := active[entry.fingerprint]
		if known && ecdsa.Verify(publicKey, digest[:], entry.r, entry.s) {
			return nil
		}
	}
	return ErrNoTrustedReleaseSignature
}

func parseReleaseSignatureEnvelope(data []byte) ([]releaseSignatureEntry, error) {
	if len(data) == 0 || len(data) > MaxReleaseSignatureEnvelopeSize {
		return nil, malformedEnvelope("size outside allowed range")
	}
	if bytes.ContainsAny(data, "\r\x00") || data[len(data)-1] != '\n' {
		return nil, malformedEnvelope("only LF-terminated text is accepted")
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 || lines[0] != ReleaseSignatureEnvelopeHeader || lines[len(lines)-1] != "" {
		return nil, malformedEnvelope("invalid header or record layout")
	}
	records := lines[1 : len(lines)-1]
	if len(records) == 0 || len(records) > MaxReleaseSignatureCount {
		return nil, malformedEnvelope("signature count outside allowed range")
	}

	entries := make([]releaseSignatureEntry, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for index, line := range records {
		entry, err := parseReleaseSignatureLine(line)
		if err != nil {
			return nil, malformedEnvelope("record %d: %v", index+1, err)
		}
		if _, duplicate := seen[entry.fingerprint]; duplicate {
			return nil, malformedEnvelope("record %d: duplicate fingerprint", index+1)
		}
		seen[entry.fingerprint] = struct{}{}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseReleaseSignatureLine(line string) (releaseSignatureEntry, error) {
	if len(line) == 0 || len(line) > MaxReleaseSignatureLineSize || strings.Count(line, "\t") != 1 {
		return releaseSignatureEntry{}, fmt.Errorf("invalid line shape")
	}
	fingerprint, encoded, _ := strings.Cut(line, "\t")
	if !validReleaseKeyFingerprint(fingerprint) {
		return releaseSignatureEntry{}, fmt.Errorf("invalid fingerprint")
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.StdEncoding.EncodeToString(signature) != encoded {
		return releaseSignatureEntry{}, fmt.Errorf("invalid canonical base64")
	}
	parsed, err := parseP256ASN1Signature(signature)
	if err != nil {
		return releaseSignatureEntry{}, err
	}
	return releaseSignatureEntry{fingerprint: fingerprint, r: parsed.R, s: parsed.S}, nil
}

func parseP256ASN1Signature(signature []byte) (ecdsaASN1Signature, error) {
	var parsed ecdsaASN1Signature
	rest, err := asn1.Unmarshal(signature, &parsed)
	if err != nil || len(rest) != 0 || parsed.R == nil || parsed.S == nil {
		return parsed, fmt.Errorf("invalid ECDSA ASN.1/DER signature")
	}
	order := elliptic.P256().Params().N
	if parsed.R.Sign() <= 0 || parsed.S.Sign() <= 0 || parsed.R.Cmp(order) >= 0 || parsed.S.Cmp(order) >= 0 {
		return parsed, fmt.Errorf("ECDSA scalar outside P-256 range")
	}
	canonical, err := asn1.Marshal(parsed)
	if err != nil || !bytes.Equal(canonical, signature) {
		return parsed, fmt.Errorf("non-canonical ECDSA ASN.1/DER signature")
	}
	return parsed, nil
}

func malformedEnvelope(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformedReleaseSignatureEnvelope, fmt.Sprintf(format, args...))
}
