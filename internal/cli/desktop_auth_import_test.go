package cli

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportDesktopAuthPayload_PersistsCredentialsAndConfig(t *testing.T) {
	prevSaveCreds := desktopAuthSaveCredentials
	prevSaveConfig := desktopAuthSaveConfig
	prevLoadConfig := desktopAuthLoadConfig
	t.Cleanup(func() {
		desktopAuthSaveCredentials = prevSaveCreds
		desktopAuthSaveConfig = prevSaveConfig
		desktopAuthLoadConfig = prevLoadConfig
	})

	var savedCreds map[string]any
	var savedConfig setup.WorkerConfig

	desktopAuthSaveCredentials = func(creds map[string]any) error {
		savedCreds = creds
		return nil
	}
	desktopAuthSaveConfig = func(cfg setup.WorkerConfig) error {
		savedConfig = cfg
		return nil
	}
	desktopAuthLoadConfig = func() (*setup.WorkerConfig, error) {
		return &setup.WorkerConfig{KnowledgeDir: "/tmp/knowledge"}, nil
	}

	result, err := importDesktopAuthPayload(strings.NewReader(`{
	  "backend_url":"https://api.autopus.co",
	  "workspace_id":"ws-123",
	  "access_token":"jwt-token",
	  "refresh_token":"refresh-token",
	  "expires_at":"2030-01-01T00:00:00Z"
	}`))
	require.NoError(t, err)

	require.NotNil(t, result)
	assert.True(t, result.OK)
	assert.Equal(t, "https://api.autopus.co", result.BackendURL)
	assert.Equal(t, "ws-123", result.WorkspaceID)

	assert.Equal(t, "jwt-token", savedCreds["access_token"])
	assert.Equal(t, "refresh-token", savedCreds["refresh_token"])
	assert.Equal(t, "2030-01-01T00:00:00Z", savedCreds["expires_at"])

	assert.Equal(t, "https://api.autopus.co", savedConfig.BackendURL)
	assert.Equal(t, "ws-123", savedConfig.WorkspaceID)
	assert.Equal(t, "/tmp/knowledge", savedConfig.KnowledgeDir)
}

func TestImportDesktopAuthPayload_RequiresCoreFields(t *testing.T) {
	_, err := importDesktopAuthPayload(strings.NewReader(`{"backend_url":"","workspace_id":"ws","access_token":"tok"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backend_url is required")
}
