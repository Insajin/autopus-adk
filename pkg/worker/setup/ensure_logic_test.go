package setup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// marshalEnsureResult serializes correctly and preserves action + data fields.
func TestMarshalEnsureResult_Ready(t *testing.T) {
	t.Parallel()

	result := &EnsureResult{
		Action: "ready",
		Data:   map[string]string{"workspace_id": "ws-marshal"},
	}
	data, err := marshalEnsureResult(result)
	require.NoError(t, err)

	s := string(data)
	assert.Contains(t, s, `"action":"ready"`)
	assert.Contains(t, s, `"ws-marshal"`)
}

func TestMarshalEnsureResult_LoginRequired(t *testing.T) {
	t.Parallel()

	result := &EnsureResult{
		Action: "login_required",
		Data: map[string]string{
			"url":        "https://auth.autopus.co/verify?code=ABC123",
			"code":       "ABC-123",
			"expires_in": "300",
		},
	}
	data, err := marshalEnsureResult(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"login_required"`)
	assert.Contains(t, string(data), "ABC-123")
}

// ensureDesktopConfig creates/updates worker config file correctly.
func TestEnsureDesktopConfig_CreatesConfig(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	err := ensureDesktopConfig("https://api.autopus.co", "ws-desktop-cfg")
	require.NoError(t, err)

	cfg, err := LoadWorkerConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://api.autopus.co", cfg.BackendURL)
	assert.Equal(t, "ws-desktop-cfg", cfg.WorkspaceID)
}

func TestEnsureDesktopConfig_PreservesExistingFields(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	// Write an existing config with an extra field.
	require.NoError(t, SaveWorkerConfig(WorkerConfig{
		BackendURL:  "https://old.api.autopus.co",
		WorkspaceID: "ws-old",
		Concurrency: 4,
	}))

	// Update only the workspace_id.
	err := ensureDesktopConfig("https://old.api.autopus.co", "ws-new")
	require.NoError(t, err)

	cfg, err := LoadWorkerConfig()
	require.NoError(t, err)
	assert.Equal(t, "ws-new", cfg.WorkspaceID)
	assert.Equal(t, 4, cfg.Concurrency, "concurrency must be preserved")
}

// saveTokenCredentials persists JWT fields to credentials.json.
func TestSaveTokenCredentials_PersistsJWT(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	resp := &TokenResponse{
		AccessToken:  "access-tok",
		RefreshToken: "refresh-tok",
		ExpiresIn:    3600,
	}
	err := saveTokenCredentials(resp, "https://api.autopus.co")
	require.NoError(t, err)

	snapshot, err := loadCredentialSnapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot.creds)
	assert.Equal(t, "jwt", snapshot.creds.AuthType)
	assert.Equal(t, "access-tok", snapshot.creds.AccessToken)
	assert.Equal(t, "refresh-tok", snapshot.creds.RefreshToken)
	// expires_at should be in the future.
	expiry, parseErr := time.Parse(time.RFC3339, snapshot.creds.ExpiresAt)
	require.NoError(t, parseErr)
	assert.True(t, expiry.After(time.Now()))
}
