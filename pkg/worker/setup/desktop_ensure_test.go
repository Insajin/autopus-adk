package setup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureDesktopRuntime_PersistsConfigWithoutDaemonStart(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "desktop-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	result, err := EnsureDesktopRuntime(context.Background(), "https://api.autopus.co", "ws-desktop")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "ready", result.Action)
	assert.Equal(t, "ws-desktop", result.Data["workspace_id"])

	cfg, err := LoadWorkerConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://api.autopus.co", cfg.BackendURL)
	assert.Equal(t, "ws-desktop", cfg.WorkspaceID)
}
