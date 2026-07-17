package selfupdate

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"time"
)

// VerifyReleaseSignature verifies a detached ECDSA P-256 (ASN.1/DER)
// signature over the SHA-256 digest of checksums against the given pinned
// key set.
//
// Verification is multi-trial: keys whose ExpiresAt has passed as of now are
// excluded from the trial set before any cryptographic attempt (pre-trial
// expiry gate). Each remaining active key is tried in turn; the signature is
// authentic the moment any active key verifies it. The `.sig` file carries no
// in-band KeyID (unlike pkg/companionmanifest.Verify's keyed lookup, see
// verify.go), because the signed artifact is a bare file rather than a
// struct with its own KeyID field — see pinnedkey.go for the rotation
// procedure this multi-trial design supports.
func VerifyReleaseSignature(checksums, sig []byte, keys []PinnedReleaseKey, now time.Time) error {
	active := ActiveKeys(keys, now)
	if len(active) == 0 {
		return errors.New("all embedded keys expired")
	}

	digest := sha256.Sum256(checksums)
	for _, pub := range active {
		if ecdsa.VerifyASN1(pub, digest[:], sig) {
			return nil
		}
	}
	return errors.New("no trusted release signing key verified")
}
