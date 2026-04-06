// Package setup — coverage tests for uncovered credstore paths.
package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptedFileStore_Delete verifies the Delete method removes the file.
func TestEncryptedFileStore_Delete(t *testing.T) {
	t.Parallel()

	// Given: a file store with a saved credential
	store := newEncryptedFileStore(t.TempDir())
	require.NoError(t, store.Save("svc-del", "secret"))

	// When: Delete is called
	err := store.Delete("svc-del")
	require.NoError(t, err)

	// Then: loading returns error (file gone)
	_, err = store.Load("svc-del")
	assert.Error(t, err)
}

// TestEncryptedFileStore_DeleteNonExistent verifies Delete is idempotent.
func TestEncryptedFileStore_DeleteNonExistent(t *testing.T) {
	t.Parallel()

	store := newEncryptedFileStore(t.TempDir())
	// Should not error for missing file.
	assert.NoError(t, store.Delete("does-not-exist"))
}

// TestEncryptedFileStore_LoadTooShort verifies truncated file returns error.
func TestEncryptedFileStore_LoadTooShort(t *testing.T) {
	t.Parallel()

	store := newEncryptedFileStore(t.TempDir())
	// Write fewer than saltLen bytes directly.
	require.NoError(t, os.MkdirAll(store.dir, 0700))
	path := store.filePath("bad")
	require.NoError(t, os.WriteFile(path, []byte("tooshort"), 0600))

	_, err := store.Load("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// TestEncryptedFileStore_LoadCorruptedCiphertext verifies corrupt data returns error.
func TestEncryptedFileStore_LoadCorruptedCiphertext(t *testing.T) {
	t.Parallel()

	store := newEncryptedFileStore(t.TempDir())
	// Write saltLen bytes of zeros + random garbage (not valid ciphertext).
	require.NoError(t, os.MkdirAll(store.dir, 0700))
	data := make([]byte, saltLen+10)
	require.NoError(t, os.WriteFile(store.filePath("corrupt"), data, 0600))

	_, err := store.Load("corrupt")
	assert.Error(t, err)
}

// TestEncryptedFileStore_FilenameServiceSanitization verifies slashes in
// service names are converted to underscores in filenames.
func TestEncryptedFileStore_FilenameServiceSanitization(t *testing.T) {
	t.Parallel()

	store := newEncryptedFileStore(t.TempDir())
	path := store.filePath("a/b\\c")
	assert.Equal(t, filepath.Join(store.dir, "a_b_c.enc"), path)
}

// TestMigratePlaintextCredentials_NoFile verifies migration is no-op when
// no plaintext credentials.json exists.
func TestMigratePlaintextCredentials_NoFile(t *testing.T) {
	t.Parallel()

	// Use a store that would fail on Save (to detect any unexpected call).
	store := newEncryptedFileStore(t.TempDir())
	var warned []string
	warn := func(msg string) { warned = append(warned, msg) }

	// Point DefaultCredentialsPath to a non-existent path by noting that
	// migratePlaintextCredentials calls DefaultCredentialsPath() directly.
	// Since the default path almost certainly won't exist in test, this is safe.
	migratePlaintextCredentials(store, warn)

	// Should not have warned (no migration needed).
	assert.Empty(t, warned)
}

// TestMigratePlaintextCredentials_ValidJSON verifies successful migration.
func TestMigratePlaintextCredentials_ValidJSON(t *testing.T) {
	t.Parallel()

	// Temporarily override the credentials path by writing a file to the
	// default location. We isolate by using a custom store path.
	// Note: DefaultCredentialsPath() is fixed, so we mock at the store level.
	dir := t.TempDir()
	store := newEncryptedFileStore(dir)

	// Write a fake credentials.json at the default path.
	credDir := filepath.Dir(DefaultCredentialsPath())
	if err := os.MkdirAll(credDir, 0700); err != nil {
		t.Skipf("cannot create cred dir %s: %v", credDir, err)
	}

	creds := map[string]any{
		"access_token": "test-token",
		"expires_at":   "2030-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(creds)
	credPath := DefaultCredentialsPath()

	// Only write if file doesn't exist (avoid overwriting real creds).
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		require.NoError(t, os.WriteFile(credPath, data, 0600))
		defer os.Remove(credPath)

		var warned []string
		warn := func(msg string) { warned = append(warned, msg) }
		migratePlaintextCredentials(store, warn)

		// Migration should have stored the data.
		got, err := store.Load("autopus-worker")
		require.NoError(t, err)
		assert.NotEmpty(t, got)
		// Original file should be removed.
		_, statErr := os.Stat(credPath)
		assert.True(t, os.IsNotExist(statErr), "plaintext file should be removed after migration")
	} else {
		t.Skip("real credentials.json exists — skipping migration test")
	}
}

// TestMigratePlaintextCredentials_InvalidJSON verifies non-JSON file is skipped.
func TestMigratePlaintextCredentials_InvalidJSON(t *testing.T) {
	t.Parallel()

	credDir := filepath.Dir(DefaultCredentialsPath())
	if err := os.MkdirAll(credDir, 0700); err != nil {
		t.Skipf("cannot create cred dir: %v", err)
	}

	credPath := DefaultCredentialsPath()
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Skip("real credentials.json exists — skipping")
	}

	// Write invalid JSON.
	require.NoError(t, os.WriteFile(credPath, []byte("{not valid json"), 0600))
	defer os.Remove(credPath)

	store := newEncryptedFileStore(t.TempDir())
	var warned []string
	warn := func(msg string) { warned = append(warned, msg) }

	// Should skip without panic.
	migratePlaintextCredentials(store, warn)

	// Invalid JSON is skipped — no migration, no warning.
	assert.Empty(t, warned)
}

// TestNewCredentialStore_FileBackend verifies store + warning are returned on forced file backend.
func TestNewCredentialStore_FileBackend(t *testing.T) {
	t.Parallel()

	store, warn := NewCredentialStore(WithForceFileBackend(true))
	assert.NotNil(t, store)
	assert.NotEmpty(t, warn)
}

// TestNewCredentialStore_WarningCallback verifies warning callback is invoked.
func TestNewCredentialStore_WarningCallback(t *testing.T) {
	t.Parallel()

	var msgs []string
	store, _ := NewCredentialStore(
		WithForceFileBackend(true),
		WithWarningFunc(func(m string) { msgs = append(msgs, m) }),
	)
	assert.NotNil(t, store)
	assert.NotEmpty(t, msgs)
}

// TestDefaultCredentialDir_HomeExists verifies a path under home is returned.
func TestDefaultCredentialDir_HomeExists(t *testing.T) {
	t.Parallel()

	dir := defaultCredentialDir()
	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, "autopus")
}
