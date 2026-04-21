package setup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withEncryptedCredentialStore(t *testing.T) {
	t.Helper()
	prev := newCredentialStoreFunc
	prevPayload := loadCredentialPayloadFunc
	newCredentialStoreFunc = func() (CredentialStore, string) {
		return newEncryptedFileStore(defaultCredentialDir()), ""
	}
	loadCredentialPayloadFunc = func() (credentialPayload, error) {
		return loadCredentialPayloadFromStore(
			newEncryptedFileStore(defaultCredentialDir()),
			"encrypted_file",
			true,
		)
	}
	t.Cleanup(func() {
		newCredentialStoreFunc = prev
		loadCredentialPayloadFunc = prevPayload
	})
}

func writeWorkerConfigFixture(t *testing.T, workspaceID, backendURL string) {
	t.Helper()
	require.NoError(t, SaveWorkerConfig(WorkerConfig{
		WorkspaceID: workspaceID,
		BackendURL:  backendURL,
	}))
}

func TestLoadDesktopSession_ReadyWithJWT(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-desktop", "https://api.autopus.co")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "desktop-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	session := LoadDesktopSession()
	assert.True(t, session.Ready)
	assert.Equal(t, "desktop-jwt", session.AccessToken)
	assert.Equal(t, "encrypted_file", session.CredentialBackend)
	assert.True(t, session.SecureStorageReady)
	assert.True(t, session.DesktopSessionReady)
}

func TestLoadDesktopSession_FailsClosedForPlaintextCredentials(t *testing.T) {
	withLegacyCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-desktop", "https://api.autopus.co")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "desktop-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	session := LoadDesktopSession()
	assert.False(t, session.Ready)
	assert.Equal(t, "secure_storage_unavailable", session.Reason)
	assert.Equal(t, "plaintext_file", session.CredentialBackend)
	assert.False(t, session.SecureStorageReady)
}

func TestLoadDesktopSession_RequiresJWTForDesktopAPIs(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-desktop", "https://api.autopus.co")
	require.NoError(t, SaveAPIKeyCredentials("acos_worker_desktop", "https://api.autopus.co"))

	session := LoadDesktopSession()
	assert.False(t, session.Ready)
	assert.Equal(t, "jwt_required_for_desktop", session.Reason)
	assert.Equal(t, "api_key", session.AuthType)
	assert.True(t, session.SecureStorageReady)
}

func TestLoadDesktopSession_PrefersSecureSnapshotOverPlaintextFallback(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-desktop", "https://api.autopus.co")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "secure-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	credPath := DefaultCredentialsPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), 0o700))
	require.NoError(t, os.WriteFile(
		credPath,
		[]byte(`{"auth_type":"jwt","access_token":"stale-jwt","expires_at":"2000-01-01T00:00:00Z"}`),
		0o600,
	))

	status := CollectStatus()
	session := LoadDesktopSession()

	assert.True(t, status.AuthValid)
	assert.Equal(t, "encrypted_file", status.CredentialBackend)
	assert.True(t, status.DesktopSessionReady)
	assert.True(t, session.Ready)
	assert.Equal(t, "encrypted_file", session.CredentialBackend)
	assert.Equal(t, "secure-jwt", session.AccessToken)
}
