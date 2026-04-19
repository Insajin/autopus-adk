package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withLegacyCredentialStore(t *testing.T) {
	t.Helper()
	prev := newCredentialStoreFunc
	newCredentialStoreFunc = func() (CredentialStore, string) { return nil, "" }
	t.Cleanup(func() { newCredentialStoreFunc = prev })
}

type failingCredentialStore struct{}

func (failingCredentialStore) Save(service, value string) error {
	return os.ErrPermission
}

func (failingCredentialStore) Load(service string) (string, error) {
	return "", os.ErrNotExist
}

func (failingCredentialStore) Delete(service string) error {
	return nil
}

func TestSaveCredentials_WritesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	withLegacyCredentialStore(t)

	creds := map[string]any{
		"access_token": "test-token",
		"expires_in":   3600,
	}
	err := SaveCredentials(creds)
	require.NoError(t, err)

	path := filepath.Join(tmp, ".config", "autopus", "credentials.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)
	assert.Equal(t, "test-token", got["access_token"])
}

func TestSaveCredentials_Permissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	withLegacyCredentialStore(t)

	err := SaveCredentials(map[string]any{"key": "val"})
	require.NoError(t, err)

	path := filepath.Join(tmp, ".config", "autopus", "credentials.json")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestSaveCredentials_ReadOnlyDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	withLegacyCredentialStore(t)

	dir := filepath.Join(tmp, ".config", "autopus")
	require.NoError(t, os.MkdirAll(dir, 0o700))

	credPath := filepath.Join(dir, "credentials.json")
	require.NoError(t, os.MkdirAll(credPath, 0o700))

	err := SaveCredentials(map[string]any{"key": "val"})
	require.Error(t, err)
}

func TestSaveCredentials_SecureStoreRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	prev := newCredentialStoreFunc
	newCredentialStoreFunc = func() (CredentialStore, string) {
		return NewCredentialStore(WithForceFileBackend(true))
	}
	t.Cleanup(func() { newCredentialStoreFunc = prev })

	err := SaveCredentials(map[string]any{
		"access_token": "secure-token",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	require.NoError(t, err)

	token, err := LoadAuthToken()
	require.NoError(t, err)
	assert.Equal(t, "secure-token", token)

	_, statErr := os.Stat(DefaultCredentialsPath())
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestLoadAuthToken_EncryptedFileFallbackAfterPrimaryMiss(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	fileStore := newEncryptedFileStore(defaultCredentialDir())
	payload := `{"access_token":"fallback-token","expires_at":"2030-01-01T00:00:00Z"}`
	require.NoError(t, fileStore.Save(workerCredentialService, payload))

	prev := newCredentialStoreFunc
	newCredentialStoreFunc = func() (CredentialStore, string) {
		return failingCredentialStore{}, ""
	}
	t.Cleanup(func() { newCredentialStoreFunc = prev })

	token, err := LoadAuthToken()
	require.NoError(t, err)
	assert.Equal(t, "fallback-token", token)
}

func TestOpenBrowser_RunsOnDarwin(t *testing.T) {
	t.Parallel()

	_ = OpenBrowser("https://example.com")
}

func TestExtractErrorCode_PlainFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{"authorization_pending", `{"error":"authorization_pending"}`, "authorization_pending"},
		{"slow_down", `{"error":"slow_down"}`, "slow_down"},
		{"wrapped format", `{"error":{"code":"authorization_pending"}}`, "authorization_pending"},
		{"empty error", `{"error":""}`, ""},
		{"no error field", `{"data":"ok"}`, ""},
		{"invalid json", `not-json`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractErrorCode([]byte(tt.body))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestUnwrap_WrappedResponse(t *testing.T) {
	t.Parallel()

	body := `{"success":true,"data":{"access_token":"tok","refresh_token":"ref","expires_in":3600,"token_type":"Bearer"}}`
	result, err := unwrap[TokenResponse]([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, "tok", result.AccessToken)
}

func TestUnwrap_DirectResponse(t *testing.T) {
	t.Parallel()

	body := `{"access_token":"direct","refresh_token":"ref","expires_in":3600,"token_type":"Bearer"}`
	result, err := unwrap[TokenResponse]([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, "direct", result.AccessToken)
}

func TestUnwrap_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := unwrap[TokenResponse]([]byte("not-json"))
	require.Error(t, err)
}
