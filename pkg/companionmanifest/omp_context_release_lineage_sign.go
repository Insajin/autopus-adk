package companionmanifest

import (
	"crypto/ed25519"
	"crypto/subtle"
	"errors"
)

// SignCanonicalOMPContextReleaseLineage signs the canonical, domain-separated
// release lineage with a self-consistent Ed25519 private key.
func SignCanonicalOMPContextReleaseLineage(
	lineage OMPContextReleaseLineageV1,
	privateKey ed25519.PrivateKey,
) ([]byte, []byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("invalid OMP context release lineage signing key")
	}
	seed := privateKey.Seed()
	defer clear(seed)
	normalizedKey := ed25519.NewKeyFromSeed(seed)
	defer clear(normalizedKey)
	if subtle.ConstantTimeCompare(privateKey, normalizedKey) != 1 {
		return nil, nil, errors.New("invalid OMP context release lineage signing key")
	}
	body, err := CanonicalOMPContextReleaseLineageBytes(lineage)
	if err != nil {
		return nil, nil, err
	}
	message, err := OMPContextReleaseLineageSigningMessage(body)
	if err != nil {
		return nil, nil, err
	}
	return body, ed25519.Sign(normalizedKey, message), nil
}
