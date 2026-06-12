package setup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDesktopSession_NotConfigured(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	// No worker config and no credentials -> fail closed as not configured.
	session := LoadDesktopSession()
	assert.False(t, session.Ready)
	assert.Equal(t, "worker_not_configured", session.Reason)
}

func TestLoadDesktopSession_AuthInvalidForExpiredJWT(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-desktop", "https://api.autopus.co")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "expired-jwt",
		"expires_at":   time.Now().Add(-time.Hour).Format(time.RFC3339),
	}))

	session := LoadDesktopSession()
	assert.False(t, session.Ready)
	assert.Equal(t, "auth_invalid", session.Reason)
}

func TestLoadDesktopSession_WorkspaceMissing(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	// Configured via backend URL only; workspace id absent.
	writeWorkerConfigFixture(t, "", "https://api.autopus.co")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "good-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	session := LoadDesktopSession()
	assert.False(t, session.Ready)
	assert.Equal(t, "workspace_missing", session.Reason)
}

func TestLoadDesktopSession_BackendMissing(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-desktop", "")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "good-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	session := LoadDesktopSession()
	assert.False(t, session.Ready)
	assert.Equal(t, "backend_missing", session.Reason)
}
