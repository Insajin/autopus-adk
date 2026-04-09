package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCredentialStore is a simple in-memory CredentialStore for tests.
type mockCredentialStore struct {
	data    map[string]string
	saveErr error
	loadErr error
}

func newMockCredStore() *mockCredentialStore {
	return &mockCredentialStore{data: make(map[string]string)}
}

func (m *mockCredentialStore) Save(service, value string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.data[service] = value
	return nil
}

func (m *mockCredentialStore) Load(service string) (string, error) {
	if m.loadErr != nil {
		return "", m.loadErr
	}
	v, ok := m.data[service]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m *mockCredentialStore) Delete(service string) error {
	delete(m.data, service)
	return nil
}

// jsonRefreshResponse encodes the cli-refresh response wrapper.
func jsonRefreshResponse(w http.ResponseWriter, success bool, token, refresh string) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": success,
		"data": map[string]any{
			"access_token":  token,
			"refresh_token": refresh,
			"expires_in":    3600,
		},
	})
}

// TestTokenRefresher_BackoffRetry_SucceedsOnSecondAttempt verifies that on a
// transient server error, the refresher retries with backoff and succeeds on
// the second attempt.
func TestTokenRefresher_BackoffRetry_SucceedsOnSecondAttempt(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call fails with 503 (retryable).
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		jsonRefreshResponse(w, true, "new-access", "new-refresh")
	}))
	defer srv.Close()

	store := newMockCredStore()
	var refreshedToken atomic.Value
	r := NewTokenRefresher(
		srv.URL,
		store,
		func() {},
		func(token string) { refreshedToken.Store(token) },
	)

	// Pre-populate store with near-expiry credentials.
	creds := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, _ = r.ForceRefresh(ctx)

	// Token must have been refreshed after retry.
	val := refreshedToken.Load()
	require.NotNil(t, val, "onTokenRefresh must be called after retry success")
	assert.Equal(t, "new-access", val.(string))
	assert.GreaterOrEqual(t, callCount.Load(), int32(2), "must have retried at least once")
}

// TestTokenRefresher_BackoffRetry_AllFail_TriggersReauth verifies that after
// max retries all fail, onReauthNeeded is called.
func TestTokenRefresher_BackoffRetry_AllFail_TriggersReauth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always fail with 503 — simulates persistent transient error.
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	store := newMockCredStore()
	var reauthCalled atomic.Bool
	r := NewTokenRefresher(
		srv.URL,
		store,
		func() { reauthCalled.Store(true) },
		nil,
	)

	creds := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, _ = r.ForceRefresh(ctx)

	assert.True(t, reauthCalled.Load(), "onReauthNeeded must be called after all retries fail")
}

// TestTokenRefresher_CredentialStore_LoadSave verifies that the new refresher
// reads/writes credentials via CredentialStore, not raw file I/O.
func TestTokenRefresher_CredentialStore_LoadSave(t *testing.T) {
	t.Parallel()

	store := newMockCredStore()
	r := NewTokenRefresher("http://unused", store, func() {}, nil)

	original := &Credentials{
		AccessToken:  "access-store",
		RefreshToken: "refresh-store",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		Email:        "user@example.com",
		Workspace:    "ws-store",
	}
	require.NoError(t, r.SaveCredentials(original))

	// Verify saved to store, not a file.
	raw, err := store.Load("autopus-worker")
	require.NoError(t, err, "credentials must be persisted in CredentialStore")
	assert.Contains(t, raw, "access-store")

	// Load round-trip.
	loaded, err := r.LoadCredentials()
	require.NoError(t, err)
	assert.Equal(t, original.AccessToken, loaded.AccessToken)
	assert.Equal(t, original.Email, loaded.Email)
}

// TestTokenRefresher_PermanentFailure_401 verifies that a 401 response
// immediately fires auth.permanent_failure and skips backoff.
func TestTokenRefresher_PermanentFailure_401(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	store := newMockCredStore()
	var permanentFailureFired atomic.Bool
	var reauthCalled atomic.Bool

	r := NewTokenRefresher(
		srv.URL,
		store,
		func() { reauthCalled.Store(true) },
		nil,
	)
	r.OnPermanentFailure(func(event string) {
		if event == "auth.permanent_failure" {
			permanentFailureFired.Store(true)
		}
	})

	creds := &Credentials{
		AccessToken:  "old",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = r.ForceRefresh(ctx)

	// 401 must not be retried — only one HTTP call.
	assert.Equal(t, int32(1), callCount.Load(), "401 must not trigger backoff retries")
	assert.True(t, permanentFailureFired.Load(), "auth.permanent_failure event must be fired")
}

// TestTokenRefresher_ForceRefresh verifies that ForceRefresh triggers an
// immediate refresh regardless of token expiry.
func TestTokenRefresher_ForceRefresh(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonRefreshResponse(w, true, "forced-token", "new-refresh")
	}))
	defer srv.Close()

	store := newMockCredStore()
	var refreshedToken atomic.Value
	r := NewTokenRefresher(
		srv.URL,
		store,
		func() {},
		func(token string) { refreshedToken.Store(token) },
	)

	// Token is NOT near expiry — normal check would skip refresh.
	creds := &Credentials{
		AccessToken:  "valid-token",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ForceRefresh must bypass the expiry check.
	_, err := r.ForceRefresh(ctx)
	require.NoError(t, err)

	val := refreshedToken.Load()
	require.NotNil(t, val, "ForceRefresh must invoke onTokenRefresh")
	assert.Equal(t, "forced-token", val.(string))
}
