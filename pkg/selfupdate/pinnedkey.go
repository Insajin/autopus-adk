package selfupdate

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// dateLayout is the UTC calendar-date format used for ExpiresAt comparisons.
// Two dateLayout strings compare lexicographically the same as
// chronologically, so expiry checks need no time-arithmetic on the hot path
// (mirrors install.sh's `date -u +%Y-%m-%d` shell comparison in verify_signature).
const dateLayout = "2006-01-02"

// PinnedReleaseKey is embedded trust material for verifying the publisher's
// release-signing ECDSA P-256 signature over checksums.txt. KeyID and
// ExpiresAt exist for audit/rotation bookkeeping only: the signed artifact is
// a bare file with no in-band key identifier, so verification never looks a
// key up by KeyID — see VerifyReleaseSignature's multi-trial semantics.
type PinnedReleaseKey struct {
	KeyID        string // audit/rotation bookkeeping only, never read from the wire
	ExpiresAt    string // UTC calendar date, "YYYY-MM-DD"
	PublicKeyPEM string // PEM-encoded SPKI (x509 PKIX), P-256 curve
}

// EmbeddedReleaseKeys is the pinned trust anchor set compiled into the auto
// binary. It is the sole source of truth for release authenticity: GitHub
// release assets (including any KeyID hint they might carry) are untrusted
// input and never influence which key is used to verify them.
//
// Rotation procedure (2-key transition window):
//  1. Generate a new ECDSA P-256 key pair. Provision the new private key as
//     an additional CI secret alongside (not replacing) the current one.
//  2. Append the new public key here as a second PinnedReleaseKey with its
//     own KeyID/ExpiresAt. Both keys are now in the active trial set: the
//     producer signs new releases with the new key, and clients still
//     running the old binary keep verifying successfully because their
//     embedded set already contains both keys.
//  3. Once the retiring key's ExpiresAt has passed, drop its entry in a
//     follow-up release.
//
// This mirrors the KeyID/ExpiresAt bookkeeping shape of
// pkg/companionmanifest.PublicKeyReceipt (see public_key_receipt.go), but
// verification here is multi-trial rather than a keyed map lookup (see
// signature.go), because a bare detached signature carries no in-band KeyID
// field to key a lookup on.
var EmbeddedReleaseKeys = []PinnedReleaseKey{
	{
		KeyID:     "10105084",
		ExpiresAt: "2028-07-17",
		PublicKeyPEM: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE1E7VSEqlwEUXAGh8uCIxYKAlFyQ3
lOdYWlCbaLtSt1WegBqHD+TjkRiLqoGcGHouS7Nwu1bjk7ZZu26bp6BnIA==
-----END PUBLIC KEY-----`,
	},
}

// ActiveKeys returns the parsed ECDSA public keys from keys whose ExpiresAt
// has not passed as of now (pre-trial expiry gate) and whose PEM SPKI parses
// as a P-256 key. A malformed ExpiresAt or PEM excludes that entry rather
// than failing the whole call, so one bad rotation entry cannot lock every
// client out of an otherwise-valid key.
func ActiveKeys(keys []PinnedReleaseKey, now time.Time) []*ecdsa.PublicKey {
	nowDate := now.UTC().Format(dateLayout)
	var active []*ecdsa.PublicKey
	for _, k := range keys {
		if _, err := time.Parse(dateLayout, k.ExpiresAt); err != nil {
			continue
		}
		if nowDate > k.ExpiresAt {
			// Expired: strictly after ExpiresAt, matching install.sh's
			// `[ "$now" \> "$EXPIRES_n" ]` lexicographic gate.
			continue
		}
		pub, err := parseECDSASPKIPEM(k.PublicKeyPEM)
		if err != nil {
			continue
		}
		active = append(active, pub)
	}
	return active
}

func parseECDSASPKIPEM(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}
	if ecdsaPub.Curve.Params().Name != "P-256" {
		return nil, errors.New("not a P-256 curve key")
	}
	return ecdsaPub, nil
}
