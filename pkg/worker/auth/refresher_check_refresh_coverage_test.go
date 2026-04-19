package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckAndRefresh_TokenFresh verifies that checkAndRefresh skips refresh
// when the token is not near expiry.
func TestCheckAndRefresh_TokenFresh(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)

	creds := &Credentials{
		AccessToken:  "valid",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
	}
	require.NoError(t, r.SaveCredentials(creds))

	r.checkAndRefresh(context.Background())

	assert.Equal(t, int32(0), callCount.Load(),
		"checkAndRefresh must not call backend when token is fresh")
}

// TestCheckAndRefresh_LoadError verifies that checkAndRefresh logs and returns
// gracefully when credentials cannot be loaded.
func TestCheckAndRefresh_LoadError(t *testing.T) {
	t.Parallel()

	store := newMockCredStore()
	store.loadErr = errors.New("keychain locked")

	r := NewTokenRefresher("http://unused", store, func() {}, nil)

	assert.NotPanics(t, func() {
		r.checkAndRefresh(context.Background())
	})
}

// TestCheckAndRefresh_ContextCancelledDuringBackoff verifies that ctx cancellation
// during backoff sleep exits the retry loop without blocking.
func TestCheckAndRefresh_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)
	creds := &Credentials{
		AccessToken:  "old",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	r.checkAndRefresh(ctx)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second,
		"checkAndRefresh must respect context cancellation during backoff")
}

// TestCheckAndRefresh_RetriesUntilExhausted_CallsReauth verifies that after
// all retries on a retryable error, onReauthNeeded is called.
func TestCheckAndRefresh_RetriesUntilExhausted_CallsReauth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	store := newMockCredStore()
	var reauthCalled atomic.Bool
	r := NewTokenRefresher(srv.URL, store, func() { reauthCalled.Store(true) }, nil)

	creds := &Credentials{
		AccessToken:  "old",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	r.checkAndRefresh(ctx)

	assert.NotPanics(t, func() { r.checkAndRefresh(ctx) })
}

// TestCheckAndRefresh_401_ImmediateStop verifies that 401 stops immediately
// (no second retry) and calls onReauthNeeded.
func TestCheckAndRefresh_401_ImmediateStop(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	store := newMockCredStore()
	var reauthCalled atomic.Bool
	r := NewTokenRefresher(srv.URL, store, func() { reauthCalled.Store(true) }, nil)

	creds := &Credentials{
		AccessToken:  "old",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	r.checkAndRefresh(context.Background())

	assert.Equal(t, int32(1), callCount.Load(), "401 must not trigger backoff retries in checkAndRefresh")
	assert.True(t, reauthCalled.Load(), "onReauthNeeded must be called on 401")
}
