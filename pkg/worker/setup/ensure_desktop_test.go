package setup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EnsureDesktopRuntime: missing workspace_id returns error immediately.
func TestEnsureDesktopRuntime_MissingWorkspaceIDReturnsError(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	result, err := EnsureDesktopRuntime(context.Background(), "https://api.autopus.co", "  ")
	require.Error(t, err)
	assert.Equal(t, "error", result.Action)
	assert.Contains(t, result.Data["message"], "workspace_id")
}

// EnsureDesktopRuntime with a ready session returns action=ready.
func TestEnsureDesktopRuntime_ReadySessionReturnsReady(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	require.NoError(t, SaveWorkerConfig(WorkerConfig{
		BackendURL:  "https://api.autopus.co",
		WorkspaceID: "ws-desktop-ensure",
	}))
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "tok-ensure",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	result, err := EnsureDesktopRuntime(context.Background(), "https://api.autopus.co", "ws-desktop-ensure")
	require.NoError(t, err)
	assert.Equal(t, "ready", result.Action)
	assert.Equal(t, "ws-desktop-ensure", result.Data["workspace_id"])
}
