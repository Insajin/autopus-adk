// Package setup — coverage tests for apikey.go.
package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestCredentials writes a temporary credentials.json at the default path.
// Returns a cleanup function that removes the file.
func writeTestCredentials(t *testing.T, creds map[string]any) func() {
	t.Helper()
	withLegacyCredentialStore(t)
	_, cleanup := isolatedHome(t)

	data, err := json.Marshal(creds)
	require.NoError(t, err)

	credPath := DefaultCredentialsPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), 0o700))
	require.NoError(t, os.WriteFile(credPath, data, 0600))
	return func() {
		_ = os.Remove(credPath)
		cleanup()
	}
}

// TestLoadAPIKey_WithAPIKey verifies LoadAPIKey returns the key when present.
func TestLoadAPIKey_WithAPIKey(t *testing.T) {
	cleanup := writeTestCredentials(t, map[string]any{
		"auth_type": "api_key",
		"api_key":   "wrk-testkey123",
	})
	defer cleanup()

	key, err := LoadAPIKey()
	require.NoError(t, err)
	assert.Equal(t, "wrk-testkey123", key)
}

// TestLoadAPIKey_WrongAuthType verifies LoadAPIKey returns empty for JWT creds.
func TestLoadAPIKey_WrongAuthType(t *testing.T) {
	cleanup := writeTestCredentials(t, map[string]any{
		"auth_type":    "jwt",
		"access_token": "some-jwt",
	})
	defer cleanup()

	key, err := LoadAPIKey()
	require.NoError(t, err)
	assert.Empty(t, key)
}

// TestLoadAPIKey_MissingFile verifies LoadAPIKey returns empty when no file.
func TestLoadAPIKey_MissingFile(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	key, err := LoadAPIKey()
	require.NoError(t, err)
	assert.Empty(t, key)
}

// TestLoadAuthToken_APIKey verifies that API key type returns the api_key field.
func TestLoadAuthToken_APIKey(t *testing.T) {
	cleanup := writeTestCredentials(t, map[string]any{
		"auth_type": "api_key",
		"api_key":   "wrk-authtoken456",
	})
	defer cleanup()

	token, err := LoadAuthToken()
	require.NoError(t, err)
	assert.Equal(t, "wrk-authtoken456", token)
}

// TestLoadAuthToken_JWT verifies that JWT type returns the access_token field.
func TestLoadAuthToken_JWT(t *testing.T) {
	cleanup := writeTestCredentials(t, map[string]any{
		"auth_type":    "jwt",
		"access_token": "eyJhbGciOiJIUzI1NiJ9.test",
	})
	defer cleanup()

	token, err := LoadAuthToken()
	require.NoError(t, err)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.test", token)
}

// TestLoadAuthToken_Empty verifies empty return when no creds configured.
func TestLoadAuthToken_Empty(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	token, err := LoadAuthToken()
	require.NoError(t, err)
	assert.Empty(t, token)
}

// TestSaveAPIKeyCredentials_WritesFile verifies SaveAPIKeyCredentials writes creds.
func TestSaveAPIKeyCredentials_WritesFile(t *testing.T) {
	withLegacyCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	credPath := DefaultCredentialsPath()
	defer func() { _ = os.Remove(credPath) }()

	err := SaveAPIKeyCredentials("wrk-mytestkey", "https://api.autopus.co")
	require.NoError(t, err)

	// Verify the file was written and contains expected fields.
	data, readErr := os.ReadFile(credPath)
	require.NoError(t, readErr)

	var creds map[string]any
	require.NoError(t, json.Unmarshal(data, &creds))
	assert.Equal(t, "api_key", creds["auth_type"])
	assert.Equal(t, "wrk-mytestkey", creds["api_key"])
	assert.Equal(t, "https://api.autopus.co", creds["backend_url"])
	assert.NotEmpty(t, creds["created_at"])
}
