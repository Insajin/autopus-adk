// Package setup tests for credential store functionality (REQ-01, REQ-02).
package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// TestCredentialStore_InterfaceExists verifies the CredentialStore interface
// has the expected Save, Load, and Delete methods.
// RED: CredentialStore type does not exist yet.
func TestCredentialStore_InterfaceExists(t *testing.T) {
	t.Parallel()

	// Given: a CredentialStore interface
	// When: we verify its method set at compile time
	// Then: interface must have Save, Load, Delete signatures

	var _ CredentialStore = (*keychainStore)(nil)
	var _ CredentialStore = (*encryptedFileStore)(nil)
}

// TestKeychainStore_SaveAndLoad verifies keychain backend stores and retrieves
// credentials correctly.
// RED: keychainStore type does not exist yet.
func TestKeychainStore_SaveAndLoad(t *testing.T) {
	// No t.Parallel() — keychain mock uses a global provider that is not goroutine-safe.
	keyring.MockInit()

	// Given: a keychain credential store
	store := newKeychainStore()

	// When: we save a credential
	err := store.Save("test-service", "test-credential-value")
	require.NoError(t, err)

	// Then: we can retrieve the same credential value
	got, err := store.Load("test-service")
	require.NoError(t, err)
	assert.Equal(t, "test-credential-value", got)
}

// TestKeychainStore_Delete verifies keychain backend deletes credentials.
// RED: keychainStore type does not exist yet.
func TestKeychainStore_Delete(t *testing.T) {
	// No t.Parallel() — keychain mock uses a global provider that is not goroutine-safe.
	keyring.MockInit()

	// Given: a keychain credential store with a saved credential
	store := newKeychainStore()
	require.NoError(t, store.Save("delete-service", "some-value"))

	// When: we delete the credential
	err := store.Delete("delete-service")
	require.NoError(t, err)

	// Then: loading the credential returns an error (not found)
	_, err = store.Load("delete-service")
	assert.Error(t, err)
}

// TestEncryptedFileStore_FallbackWhenKeychainUnavailable verifies that the
// system falls back to encrypted file store when keychain is unavailable.
// RED: encryptedFileStore and NewCredentialStore do not exist yet.
func TestEncryptedFileStore_FallbackWhenKeychainUnavailable(t *testing.T) {
	t.Parallel()

	// Given: keychain is unavailable (forced via option)
	store, warn := NewCredentialStore(WithForceFileBackend(true))

	// When: a credential store is created
	// Then: warn message must be non-empty indicating fallback
	assert.NotNil(t, store)
	assert.NotEmpty(t, warn, "startup warning must be emitted when keychain unavailable")
}

// TestEncryptedFileStore_StartupWarning verifies that a startup warning is emitted
// when falling back to encrypted file store.
// RED: NewCredentialStore with WarningFunc callback does not exist yet.
func TestEncryptedFileStore_StartupWarning(t *testing.T) {
	t.Parallel()

	// Given: a credential store that forces file backend
	var warningMessages []string
	_, _ = NewCredentialStore(
		WithForceFileBackend(true),
		WithWarningFunc(func(msg string) {
			warningMessages = append(warningMessages, msg)
		}),
	)

	// Then: at least one warning was emitted
	assert.NotEmpty(t, warningMessages, "startup warning must be emitted on fallback")
}

// TestEncryptedFileStore_AES256GCMRoundtrip verifies AES-256-GCM encryption
// and decryption roundtrip for credential values.
// RED: encryptedFileStore and its encrypt/decrypt methods do not exist yet.
func TestEncryptedFileStore_AES256GCMRoundtrip(t *testing.T) {
	t.Parallel()

	// Given: an encrypted file store
	store := newEncryptedFileStore(t.TempDir())

	// When: we save a credential
	plaintext := "super-secret-api-key-12345"
	err := store.Save("roundtrip-key", plaintext)
	require.NoError(t, err)

	// Then: we can load and get the original plaintext back
	got, err := store.Load("roundtrip-key")
	require.NoError(t, err)
	assert.Equal(t, plaintext, got, "decrypted value must match original plaintext")

	// And: the raw on-disk content must NOT contain the plaintext
	rawBytes, err := store.rawBytes("roundtrip-key")
	require.NoError(t, err)
	assert.NotContains(t, string(rawBytes), plaintext, "raw file must not contain plaintext")
}
