// Package setup — coverage tests for status.go functions.
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

// writeCredentials writes temporary credentials.json and returns cleanup func.
// Skips the test if real credentials already exist.
func writeCredentials(t *testing.T, creds map[string]any) func() {
	t.Helper()
	withLegacyCredentialStore(t)
	_, cleanup := isolatedHome(t)
	credPath := DefaultCredentialsPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), 0o700))
	data, err := json.Marshal(creds)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credPath, data, 0600))
	return func() {
		_ = os.Remove(credPath)
		cleanup()
	}
}

// TestCheckAuthValidity_APIKey verifies API key auth is treated as valid.
func TestCheckAuthValidity_APIKey(t *testing.T) {
	cleanup := writeCredentials(t, map[string]any{
		"api_key":   "wrk-testkey",
		"auth_type": "api_key",
	})
	defer cleanup()

	valid, authType := checkAuthValidity()
	assert.True(t, valid)
	assert.Equal(t, "api_key", authType)
}

// TestCheckAuthValidity_ValidJWT verifies unexpired JWT is treated as valid.
func TestCheckAuthValidity_ValidJWT(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	cleanup := writeCredentials(t, map[string]any{
		"access_token": "eyJhbGciOiJIUzI1NiJ9.test",
		"expires_at":   future,
	})
	defer cleanup()

	valid, authType := checkAuthValidity()
	assert.True(t, valid)
	assert.Equal(t, "jwt", authType)
}

// TestCheckAuthValidity_ExpiredJWT verifies expired JWT is treated as invalid.
func TestCheckAuthValidity_ExpiredJWT(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	cleanup := writeCredentials(t, map[string]any{
		"access_token": "eyJhbGciOiJIUzI1NiJ9.test",
		"expires_at":   past,
	})
	defer cleanup()

	valid, authType := checkAuthValidity()
	assert.False(t, valid)
	assert.Equal(t, "jwt", authType)
}

// TestCheckAuthValidity_NoFile verifies missing file returns false/none.
func TestCheckAuthValidity_NoFile(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	valid, authType := checkAuthValidity()
	assert.False(t, valid)
	assert.Equal(t, "none", authType)
}

// TestCheckAuthValidity_NoExpiryJWT verifies JWT with no expiry is valid.
func TestCheckAuthValidity_NoExpiryJWT(t *testing.T) {
	cleanup := writeCredentials(t, map[string]any{
		"access_token": "eyJhbGciOiJIUzI1NiJ9.test",
	})
	defer cleanup()

	valid, authType := checkAuthValidity()
	assert.True(t, valid)
	assert.Equal(t, "jwt", authType)
}

// TestCheckAuthValidity_UnparseableExpiry verifies unparseable expiry is treated as valid.
func TestCheckAuthValidity_UnparseableExpiry(t *testing.T) {
	cleanup := writeCredentials(t, map[string]any{
		"access_token": "eyJhbGciOiJIUzI1NiJ9.test",
		"expires_at":   "not-a-date",
	})
	defer cleanup()

	valid, authType := checkAuthValidity()
	assert.True(t, valid)
	assert.Equal(t, "jwt", authType)
}

// TestCollectStatus_NoConfig verifies CollectStatus works with no config file.
func TestCollectStatus_NoConfig(t *testing.T) {
	t.Parallel()

	// CollectStatus reads the default config path — if it doesn't exist, Configured=false.
	// This test verifies it does not panic.
	status := CollectStatus()
	// DaemonRunning may be true or false depending on the environment.
	// Just verify the struct is populated.
	assert.IsType(t, WorkerStatus{}, status)
}

func TestCollectStatus_DesktopSessionReady(t *testing.T) {
	withEncryptedCredentialStore(t)
	_, cleanup := isolatedHome(t)
	defer cleanup()

	writeWorkerConfigFixture(t, "ws-ready", "https://api.autopus.co")
	require.NoError(t, SaveCredentials(map[string]any{
		"auth_type":    "jwt",
		"access_token": "desktop-jwt",
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}))

	status := CollectStatus()
	assert.Equal(t, "encrypted_file", status.CredentialBackend)
	assert.True(t, status.SecureStorageReady)
	assert.True(t, status.DesktopSessionReady)
}

// TestDefaultCredentialsPath verifies path contains autopus.
func TestDefaultCredentialsPath(t *testing.T) {
	t.Parallel()

	p := DefaultCredentialsPath()
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "autopus")
}
