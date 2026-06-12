package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPathCredentialStore_BlankPathReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewPathCredentialStore("   "))
}

func TestPathCredentialStore_SaveLoadDelete(t *testing.T) {
	t.Parallel()

	// Nested dir to exercise MkdirAll inside Save.
	path := filepath.Join(t.TempDir(), "nested", "dir", "credentials.json")
	store := NewPathCredentialStore(path)
	require.NotNil(t, store)

	require.NoError(t, store.Save("svc", `{"api_key":"acos_x"}`))

	// File is written with 0600 permissions.
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	loaded, err := store.Load("svc")
	require.NoError(t, err)
	assert.Equal(t, `{"api_key":"acos_x"}`, loaded)

	require.NoError(t, store.Delete("svc"))
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	// Delete is idempotent: deleting a missing file is not an error.
	require.NoError(t, store.Delete("svc"))
}

func TestPathCredentialStore_LoadMissingFileErrors(t *testing.T) {
	t.Parallel()

	store := NewPathCredentialStore(filepath.Join(t.TempDir(), "absent.json"))
	_, err := store.Load("svc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read credentials")
}

func TestLoadRawCredentialsFromPath_ParsesExplicitFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "creds.json")
	require.NoError(t, os.WriteFile(path,
		[]byte(`{"auth_type":"api_key","api_key":"acos_token","workspace":"ws-7"}`), 0o600))

	creds, err := loadRawCredentialsFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, "api_key", creds.AuthType)
	assert.Equal(t, "acos_token", creds.APIKey)
	assert.Equal(t, "ws-7", creds.Workspace)
}

func TestLoadRawCredentialsFromPath_InvalidJSONErrors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	_, err := loadRawCredentialsFromPath(path)
	require.Error(t, err)
}

func TestLoadCredentialBytesFromPath_ExplicitPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raw.json")
	require.NoError(t, os.WriteFile(path, []byte("payload-bytes"), 0o600))

	data, err := loadCredentialBytesFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, "payload-bytes", string(data))
}
