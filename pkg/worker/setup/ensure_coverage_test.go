// Package setup — coverage tests for ensure.go.
package setup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveTokenCredentials_WritesExpiry verifies saveTokenCredentials writes
// expires_at when ExpiresIn > 0.
func TestSaveTokenCredentials_WritesExpiry(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	credPath := DefaultCredentialsPath()
	defer func() { _ = os.Remove(credPath) }()

	tokenResp := &TokenResponse{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresIn:    3600,
	}

	err := saveTokenCredentials(tokenResp, "https://api.autopus.co")
	require.NoError(t, err)

	data, readErr := os.ReadFile(credPath)
	require.NoError(t, readErr)

	var creds map[string]any
	require.NoError(t, json.Unmarshal(data, &creds))
	assert.Equal(t, "jwt", creds["auth_type"])
	assert.Equal(t, "test-access", creds["access_token"])
	assert.Equal(t, "test-refresh", creds["refresh_token"])
	assert.NotEmpty(t, creds["expires_at"])
}

// TestSaveTokenCredentials_NoExpiry verifies expires_at is omitted when ExpiresIn=0.
func TestSaveTokenCredentials_NoExpiry(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	credPath := DefaultCredentialsPath()
	defer func() { _ = os.Remove(credPath) }()

	tokenResp := &TokenResponse{
		AccessToken:  "access-no-expiry",
		RefreshToken: "refresh-no-expiry",
		ExpiresIn:    0,
	}

	err := saveTokenCredentials(tokenResp, "https://api.autopus.co")
	require.NoError(t, err)

	data, readErr := os.ReadFile(credPath)
	require.NoError(t, readErr)

	var creds map[string]any
	require.NoError(t, json.Unmarshal(data, &creds))
	assert.Equal(t, "jwt", creds["auth_type"])
	_, hasExpiry := creds["expires_at"]
	assert.False(t, hasExpiry, "expires_at should not be set when ExpiresIn=0")
}

// TestTryRefreshCredentials_NoCredentials verifies false returned when no creds file.
func TestTryRefreshCredentials_NoCredentials(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	ok := tryRefreshCredentials(context.Background(), "https://api.autopus.co")
	assert.False(t, ok)
}

// TestTryRefreshCredentials_NoRefreshToken verifies false returned when no refresh token.
func TestTryRefreshCredentials_NoRefreshToken(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	credPath := DefaultCredentialsPath()
	defer func() { _ = os.Remove(credPath) }()

	// Write creds without refresh_token.
	creds := map[string]any{"access_token": "test", "auth_type": "jwt"}
	data, _ := json.Marshal(creds)
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), 0700))
	require.NoError(t, os.WriteFile(credPath, data, 0600))

	ok := tryRefreshCredentials(context.Background(), "https://api.autopus.co")
	assert.False(t, ok)
}

// TestTryRefreshCredentials_ServerError verifies false returned on server error.
func TestTryRefreshCredentials_ServerError(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()
	withLegacyCredentialStore(t)

	credPath := DefaultCredentialsPath()
	defer func() { _ = os.Remove(credPath) }()

	// Server that fails token refresh.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	// Write creds with a refresh_token.
	creds := map[string]any{
		"access_token":  "old-access",
		"refresh_token": "old-refresh",
		"expires_at":    time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	}
	data, _ := json.Marshal(creds)
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), 0700))
	require.NoError(t, os.WriteFile(credPath, data, 0600))

	ok := tryRefreshCredentials(context.Background(), srv.URL)
	assert.False(t, ok)
}
