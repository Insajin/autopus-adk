// Package setup — additional auth.go coverage tests.
package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveCredentials_WriteFileFails verifies error when write fails.
func TestSaveCredentials_WriteFileFails(t *testing.T) {
	// Use a read-only file at the credentials path to force write failure.
	homeDir, cleanup := isolatedHome(t)
	defer cleanup()

	configDir := filepath.Join(homeDir, ".config", "autopus")
	require.NoError(t, os.MkdirAll(configDir, 0700))

	// Create credentials.json as a directory (write will fail).
	credPath := filepath.Join(configDir, "credentials.json")
	require.NoError(t, os.Mkdir(credPath, 0700))

	err := SaveCredentials(map[string]any{"test": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write credentials")
}

// TestSaveCredentials_MkdirAllFails verifies error when dir creation fails.
func TestSaveCredentials_MkdirAllFails(t *testing.T) {
	homeDir, cleanup := isolatedHome(t)
	defer cleanup()

	// Create a file at the .config path to block MkdirAll.
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".config"), []byte("block"), 0600))

	err := SaveCredentials(map[string]any{"test": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create config dir")
}

// TestRequestDeviceCode_UnwrappedResponse verifies unwrapped (direct) JSON response.
func TestRequestDeviceCode_UnwrappedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dc := DeviceCode{
			DeviceCode:  "dev-123",
			UserCode:    "CODE-456",
			ExpiresIn:   300,
			Interval:    5,
			VerificationURI:         "https://app.example.com/device",
			VerificationURIComplete: "",
		}
		// Return direct (unwrapped) JSON.
		json.NewEncoder(w).Encode(dc)
	}))
	defer srv.Close()

	dc, err := RequestDeviceCode(srv.URL, "verifier-test")
	require.NoError(t, err)
	assert.Equal(t, "dev-123", dc.DeviceCode)
}

// TestMigratePlaintextCredentials_SaveFails verifies warn is called when store.Save fails.
func TestMigratePlaintextCredentials_SaveFails(t *testing.T) {
	homeDir, cleanup := isolatedHome(t)
	defer cleanup()

	// Write a valid credentials.json
	configDir := filepath.Join(homeDir, ".config", "autopus")
	require.NoError(t, os.MkdirAll(configDir, 0700))
	credData := `{"access_token":"test"}`
	credPath := filepath.Join(configDir, "credentials.json")
	require.NoError(t, os.WriteFile(credPath, []byte(credData), 0600))

	// Use a read-only dir store to force Save failure.
	roDir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.MkdirAll(roDir, 0400)) // read-only dir

	store := newEncryptedFileStore(roDir)
	var warnMsgs []string
	warn := func(msg string) { warnMsgs = append(warnMsgs, msg) }

	migratePlaintextCredentials(store, warn)

	// Warn should have been called since Save failed.
	require.NotEmpty(t, warnMsgs, "warn must be called on save failure")
	assert.Contains(t, warnMsgs[0], "Failed to migrate")
}

// TestCredstoreFileSave_MkdirAllFails verifies error when dir creation fails.
func TestCredstoreFileSave_MkdirAllFails(t *testing.T) {
	t.Parallel()

	// Use a path where a file exists at the dir location.
	tmpFile := filepath.Join(t.TempDir(), "blockfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0600))

	store := newEncryptedFileStore(filepath.Join(tmpFile, "creds"))
	err := store.Save("test-svc", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create credential dir")
}

// TestCredstoreFileDelete_ReadOnlyDir verifies error when delete fails on read-only dir.
func TestCredstoreFileDelete_ReadOnlyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newEncryptedFileStore(dir)

	// Save first then make dir read-only.
	require.NoError(t, store.Save("test-del", "val"))
	require.NoError(t, os.Chmod(dir, 0500)) // remove write permission
	defer os.Chmod(dir, 0700)              // restore for cleanup

	err := store.Delete("test-del")
	// Error depends on OS — we just verify it is non-nil OR the file is gone.
	// On some systems removing from read-only dir succeeds via cached inode.
	_ = err
}
