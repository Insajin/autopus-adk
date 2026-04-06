// Package setup — coverage tests for AES-256-GCM encrypt/decrypt error paths.
package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptGCM_InvalidKeyLength verifies error on wrong key length.
// AES requires 16, 24, or 32 byte keys.
func TestEncryptGCM_InvalidKeyLength(t *testing.T) {
	t.Parallel()

	// Given: a key of invalid length (7 bytes — not 16/24/32)
	badKey := make([]byte, 7)

	// When: encryptGCM is called
	_, err := encryptGCM(badKey, []byte("plaintext"))

	// Then: an error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create cipher")
}

// TestDecryptGCM_InvalidKeyLength verifies error on wrong key length.
func TestDecryptGCM_InvalidKeyLength(t *testing.T) {
	t.Parallel()

	badKey := make([]byte, 7)
	// Use a data buffer long enough to pass the length check (12+ bytes for nonce).
	data := make([]byte, 20)

	_, err := decryptGCM(badKey, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create cipher")
}

// TestDecryptGCM_TamperedCiphertext verifies authentication failure on tampered data.
func TestDecryptGCM_TamperedCiphertext(t *testing.T) {
	t.Parallel()

	// Given: a valid 32-byte key and plaintext
	key := make([]byte, 32)
	plaintext := []byte("secret message")

	// Encrypt first
	ciphertext, err := encryptGCM(key, plaintext)
	require.NoError(t, err)

	// Tamper with the ciphertext (flip last byte)
	ciphertext[len(ciphertext)-1] ^= 0xFF

	// When: decryptGCM is called on tampered data
	_, err = decryptGCM(key, ciphertext)

	// Then: an error is returned (authentication tag mismatch)
	require.Error(t, err)
}

// TestMachinePassphrase_ReturnsNonEmpty verifies machinePassphrase returns a string.
func TestMachinePassphrase_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	p := machinePassphrase()
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "@", "passphrase should contain username@hostname")
}
