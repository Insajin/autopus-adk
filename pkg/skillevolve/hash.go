package skillevolve

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return hashString("")
	}
	return hashString(string(body))
}

func shortDigest(value string) string {
	digest := strings.TrimPrefix(hashString(value), "sha256:")
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}
