package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fileCredStore wraps a file path as a CredentialStore for legacy tests.
// Credentials are stored as JSON under the "autopus-worker" key in-memory.
type fileCredStore struct {
	path string
	data map[string]string
}

func newFileCredStore(path string) *fileCredStore {
	return &fileCredStore{path: path, data: make(map[string]string)}
}

func (f *fileCredStore) Save(service, value string) error {
	f.data[service] = value
	return nil
}

func (f *fileCredStore) Load(service string) (string, error) {
	v, ok := f.data[service]
	if !ok {
		return "", &credNotFoundError{service: service}
	}
	return v, nil
}

func (f *fileCredStore) Delete(service string) error {
	delete(f.data, service)
	return nil
}

type credNotFoundError struct{ service string }

func (e *credNotFoundError) Error() string { return "credential not found: " + e.service }

func TestLoadSaveCredentials_Roundtrip(t *testing.T) {
	t.Parallel()
	store := newFileCredStore("/unused")

	r := NewTokenRefresher("http://unused", store, func() {}, nil)

	original := &Credentials{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
		Email:        "test@example.com",
		Workspace:    "ws-1",
	}

	err := r.SaveCredentials(original)
	require.NoError(t, err)

	loaded, err := r.LoadCredentials()
	require.NoError(t, err)

	assert.Equal(t, original.AccessToken, loaded.AccessToken)
	assert.Equal(t, original.RefreshToken, loaded.RefreshToken)
	assert.Equal(t, original.Email, loaded.Email)
	assert.Equal(t, original.Workspace, loaded.Workspace)
	assert.True(t, original.ExpiresAt.Equal(loaded.ExpiresAt))
}

func TestLoadCredentials_FileNotFound(t *testing.T) {
	t.Parallel()
	// Empty store → Load returns error.
	store := newFileCredStore("/nonexistent")
	r := NewTokenRefresher("http://unused", store, func() {}, nil)
	_, err := r.LoadCredentials()
	assert.Error(t, err)
}

func TestRefresh_FiresBeforeExpiry(t *testing.T) {
	t.Parallel()

	// Mock response matches the cli-refresh wrapper format the refresher decodes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			},
		})
	}))
	defer srv.Close()

	store := newFileCredStore("/unused")
	var refreshedToken atomic.Value
	r := NewTokenRefresher(srv.URL, store, func() {}, func(token string) {
		refreshedToken.Store(token)
	})

	// Save credentials that expire very soon (within the 5-minute window).
	nearExpiry := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Minute),
		Email:        "user@test.com",
		Workspace:    "ws",
	}
	require.NoError(t, r.SaveCredentials(nearExpiry))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run checkAndRefresh directly instead of Start to avoid timer waits.
	r.checkAndRefresh(ctx)

	val := refreshedToken.Load()
	require.NotNil(t, val, "onTokenRefresh should have been called")
	assert.Equal(t, "new-access", val.(string))
}

func TestRefresh_OnReauthNeeded(t *testing.T) {
	t.Parallel()

	// Server always returns 401 to simulate refresh failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	store := newFileCredStore("/unused")
	var reauthCalled atomic.Bool
	r := NewTokenRefresher(srv.URL, store, func() {
		reauthCalled.Store(true)
	}, nil)

	// Save near-expiry credentials.
	nearExpiry := &Credentials{
		AccessToken:  "old",
		RefreshToken: "old-ref",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(nearExpiry))

	r.checkAndRefresh(context.Background())

	assert.True(t, reauthCalled.Load(), "onReauthNeeded should be called on refresh failure")
}

func TestSaveCredentials_StoreRoundtrip(t *testing.T) {
	t.Parallel()
	store := newFileCredStore("/unused")

	r := NewTokenRefresher("http://unused", store, func() {}, nil)
	err := r.SaveCredentials(&Credentials{AccessToken: "tok"})
	require.NoError(t, err)

	loaded, err := r.LoadCredentials()
	require.NoError(t, err)
	assert.Equal(t, "tok", loaded.AccessToken)
}
