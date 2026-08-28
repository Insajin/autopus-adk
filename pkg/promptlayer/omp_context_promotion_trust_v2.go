package promptlayer

import (
	"crypto/ed25519"
	"encoding/base64"
)

const (
	ompContextPromotionPublicKey2026Q3K1Base64 = "2ZO4NEHN+2yUw3huo8ZIXp/ITGd6WMN+EyiQVc9a3y8="
	ompContextPromotionPublicKey2026Q3K2Base64 = "vYEuNBwZoVzxi2WcRFUbYvdCXrY0s7XGy8K2qDilDPs="
	ompContextPromotionPublicKey2026Q3K3Base64 = "YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM="
)

func mustOMPContextPromotionPublicKeyV2(encoded string) ed25519.PublicKey {
	encoding := base64.StdEncoding.Strict()
	raw, err := encoding.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize || encoding.EncodeToString(raw) != encoded {
		panic("invalid committed OMP context promotion public key")
	}
	return ed25519.PublicKey(raw)
}

var ompContextPromotionPublicKeysV2 = map[string]ed25519.PublicKey{
	OMPContextPromotionKeyID2026Q3K1: mustOMPContextPromotionPublicKeyV2(ompContextPromotionPublicKey2026Q3K1Base64),
	OMPContextPromotionKeyID2026Q3K2: mustOMPContextPromotionPublicKeyV2(ompContextPromotionPublicKey2026Q3K2Base64),
	OMPContextPromotionKeyID2026Q3K3: mustOMPContextPromotionPublicKeyV2(ompContextPromotionPublicKey2026Q3K3Base64),
}

var ompContextPromotionRevokedKeysV2 = map[string]bool{}

func committedOMPContextPromotionPublicKeysV2() map[string]ed25519.PublicKey {
	result := make(map[string]ed25519.PublicKey, len(ompContextPromotionPublicKeysV2))
	for keyID, publicKey := range ompContextPromotionPublicKeysV2 {
		result[keyID] = append(ed25519.PublicKey(nil), publicKey...)
	}
	return result
}
