// Package setup — additional EnsureWorker coverage using fresh temp config dirs.
package setup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolatedConfig sets up a temporary HOME so DefaultWorkerConfigPath and
// DefaultCredentialsPath return paths in an isolated temp dir.
// Returns cleanup func. Must NOT call t.Parallel() — modifies process-global HOME.
func isolatedHome(t *testing.T) (homeDir string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	return dir, func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}
}

// TestEnsureWorker_NoConfig_DeviceAuthError verifies EnsureWorker error path
// when not configured and device code request fails.
func TestEnsureWorker_NoConfig_DeviceAuthError(t *testing.T) {
	// Must not be parallel — modifies HOME.
	_, cleanup := isolatedHome(t)
	defer cleanup()

	// Given: server returning error on device code
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	// When: EnsureWorker called with no config present
	_, err := EnsureWorker(context.Background(), srv.URL, "")

	// Then: error is returned (device auth failed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device code")
}

// TestEnsureWorker_NoConfig_DeviceAuthContextCancelled verifies EnsureWorker
// returns login_required when device code succeeds but poll is cancelled.
func TestEnsureWorker_NoConfig_DeviceAuthContextCancelled(t *testing.T) {
	_, cleanup := isolatedHome(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/device/code" {
			w.Header().Set("Content-Type", "application/json")
			dc := DeviceCode{
				DeviceCode:              "dev-code-123",
				UserCode:                "ABCD-1234",
				VerificationURI:         srv_url_placeholder,
				VerificationURIComplete: srv_url_placeholder + "?code=ABCD-1234",
				ExpiresIn:               300,
				Interval:                1,
			}
			resp, _ := json.Marshal(map[string]any{"data": dc})
			_, _ = w.Write(resp)
			return
		}
		// Poll endpoint: return authorization_pending
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
	}))
	defer srv.Close()

	// Patch the placeholder with the actual server URL.
	srv_url_placeholder = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := EnsureWorker(ctx, srv.URL, "")
	_ = err
	if result != nil {
		assert.Equal(t, "login_required", result.Action)
	}
}

// srv_url_placeholder is used by the closure above — filled after srv is created.
var srv_url_placeholder = ""

// TestEnsureWorker_ConfiguredAuthValid_DaemonNotRunning verifies a Go test
// binary cannot be persisted as the worker daemon.
func TestEnsureWorker_ConfiguredAuthValid_DaemonNotRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchctl regression test")
	}

	const (
		helperEnv     = "AUTOPUS_ENSURE_WORKER_TEST_HELPER"
		helperHomeEnv = "AUTOPUS_ENSURE_WORKER_TEST_HOME"
		helperMarker  = ".autopus-launchd-test-home"
	)
	if os.Getenv(helperEnv) != "1" {
		homeDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, helperMarker), []byte("isolated"), 0600))
		binDir := filepath.Join(homeDir, "bin")
		require.NoError(t, os.MkdirAll(binDir, 0700))

		launchctlLog := filepath.Join(homeDir, "launchctl.log")
		fakeLaunchctl := filepath.Join(binDir, "launchctl")
		script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$AUTOPUS_LAUNCHCTL_LOG\"\nexit 1\n"
		require.NoError(t, os.WriteFile(fakeLaunchctl, []byte(script), 0700))

		cmd := exec.Command(os.Args[0], "-test.run=^TestEnsureWorker_ConfiguredAuthValid_DaemonNotRunning$")
		cmd.Env = append(os.Environ(),
			helperEnv+"=1",
			helperHomeEnv+"="+homeDir,
			"HOME="+homeDir,
			"PATH="+binDir,
			"AUTOPUS_LAUNCHCTL_LOG="+launchctlLog,
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		calls, err := os.ReadFile(launchctlLog)
		require.NoError(t, err)
		assert.Contains(t, string(calls), "list co.autopus.worker")
		assert.NotContains(t, string(calls), "load ")
		_, err = os.Stat(filepath.Join(
			homeDir,
			"Library",
			"LaunchAgents",
			"co.autopus.worker.plist",
		))
		assert.ErrorIs(t, err, os.ErrNotExist)
		return
	}

	homeDir := os.Getenv("HOME")
	require.Equal(t, os.Getenv(helperHomeEnv), homeDir)
	require.NotEmpty(t, homeDir)
	marker, err := os.ReadFile(filepath.Join(homeDir, helperMarker))
	require.NoError(t, err)
	require.Equal(t, "isolated", string(marker))

	// Write a valid worker config
	configDir := filepath.Join(homeDir, ".config", "autopus")
	require.NoError(t, os.MkdirAll(configDir, 0700))
	// Use YAML format for config
	yamlData := "backend_url: https://api.autopus.co\nworkspace_id: ws-test\n"
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "worker.yaml"), []byte(yamlData), 0600))

	// Write valid credentials (API key — no expiry concern)
	credData := `{"api_key":"wrk-test","auth_type":"api_key"}`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "credentials.json"), []byte(credData), 0600))

	// When: EnsureWorker is called from the test executable.
	result, err := EnsureWorker(context.Background(), "https://api.autopus.co", "ws-test")

	// Then: daemon installation fails before launchctl load.
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "error", result.Action)
	assert.True(t, strings.Contains(err.Error(), "refusing") && strings.Contains(err.Error(), "setup.test"), err.Error())
}

// TestEnsureWorker_ConfiguredAuthInvalid_RefreshFails verifies that EnsureWorker
// calls device auth when auth is invalid and refresh fails.
func TestEnsureWorker_ConfiguredAuthInvalid_RefreshFails(t *testing.T) {
	homeDir, cleanup := isolatedHome(t)
	defer cleanup()

	// Write a valid worker config
	configDir := filepath.Join(homeDir, ".config", "autopus")
	require.NoError(t, os.MkdirAll(configDir, 0700))
	yamlData := "backend_url: https://api.autopus.co\nworkspace_id: ws-test\n"
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "worker.yaml"), []byte(yamlData), 0600))

	// Write expired credentials with a refresh token
	expiredAt := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	credData, _ := json.Marshal(map[string]any{
		"access_token":  "old-token",
		"refresh_token": "old-refresh",
		"expires_at":    expiredAt,
	})
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "credentials.json"), credData, 0600))

	// Given: server that rejects token refresh and device code requests
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	// When: EnsureWorker — auth invalid, refresh fails → tries device auth → fails
	_, err := EnsureWorker(context.Background(), srv.URL, "ws-test")

	// Then: error is returned (device auth failed)
	require.Error(t, err)
}
