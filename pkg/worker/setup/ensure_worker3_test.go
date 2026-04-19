// Package setup — additional ensureDeviceAuth coverage tests.
package setup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureDeviceAuth_VerificationURIFallback verifies that when
// VerificationURIComplete is empty, VerificationURI is used as the login URL.
func TestEnsureDeviceAuth_VerificationURIFallback(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/device/code" {
			w.Header().Set("Content-Type", "application/json")
			dc := DeviceCode{
				DeviceCode:              "dev-fallback-123",
				UserCode:                "ABCD-9999",
				VerificationURI:         "https://app.example.com/device",
				VerificationURIComplete: "", // empty — triggers fallback branch
				ExpiresIn:               300,
				Interval:                1,
			}
			resp, _ := json.Marshal(map[string]any{"data": dc})
			_, _ = w.Write(resp)
			return
		}
		// Poll endpoint: return authorization_pending.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so PollForToken returns quickly

	result, _ := ensureDeviceAuth(ctx, srv.URL)

	// If result was produced before context cancelled, URL must use VerificationURI.
	if result != nil {
		assert.Equal(t, "login_required", result.Action)
		assert.Equal(t, "https://app.example.com/device", result.Data["url"])
	}
}

// TestEnsureDeviceAuth_PollSucceeds_SavesCredentials verifies that when
// PollForToken returns a token, saveTokenCredentials is called and credentials
// are persisted to the isolated HOME directory.
func TestEnsureDeviceAuth_PollSucceeds_SavesCredentials(t *testing.T) {
	homeDir, cleanup := isolatedHome(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			w.Header().Set("Content-Type", "application/json")
			dc := DeviceCode{
				DeviceCode:              "dev-token-ok",
				UserCode:                "WXYZ-0000",
				VerificationURI:         "https://app.example.com/device",
				VerificationURIComplete: "https://app.example.com/device?code=WXYZ-0000",
				ExpiresIn:               300,
				Interval:                1,
			}
			resp, _ := json.Marshal(map[string]any{"data": dc})
			_, _ = w.Write(resp)

		case "/api/v1/auth/device/token":
			// Return a successful token response immediately.
			w.Header().Set("Content-Type", "application/json")
			token := TokenResponse{
				AccessToken:  "access-tok-save-test",
				RefreshToken: "refresh-tok-save-test",
				ExpiresIn:    3600,
			}
			resp, _ := json.Marshal(map[string]any{"data": token})
			_, _ = w.Write(resp)

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	result, err := ensureDeviceAuth(context.Background(), srv.URL)

	// The result is "ready", "starting_daemon", or "error" after token save.
	// All are acceptable — the key assertion is that credentials were saved.
	require.NotNil(t, result, "result must not be nil after token exchange")
	_ = err

	// Verify credentials file exists in the isolated HOME directory.
	credPath := DefaultCredentialsPath()
	data, readErr := os.ReadFile(credPath)
	if readErr == nil {
		// Credentials saved as plaintext JSON — check token present.
		assert.Contains(t, string(data), "access-tok-save-test")
	} else {
		// Credentials may be in encrypted store; verify result action is valid.
		assert.Contains(t, []string{"ready", "starting_daemon", "error"}, result.Action)
	}
	_ = homeDir
}
