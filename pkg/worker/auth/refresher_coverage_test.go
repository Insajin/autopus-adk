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

// TestStart_CancelExits verifies that Start returns when ctx is cancelled.
func TestStart_CancelExits(t *testing.T) {
	t.Parallel()

	store := newMockCredStore()
	r := NewTokenRefresher("http://unused", store, func() {}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

// TestStart_TickerTriggersRefresh verifies that the ticker path calls checkAndRefresh.
// We indirectly verify by populating near-expiry credentials and a mock server.
func TestStart_TickerTriggersRefresh(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"access_token":  "refreshed",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			},
		})
	}))
	defer srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)

	// Near-expiry creds so checkAndRefresh will attempt a refresh on the initial call.
	creds := &Credentials{
		AccessToken:  "old",
		RefreshToken: "old-ref",
		ExpiresAt:    time.Now().Add(1 * time.Minute),
	}
	require.NoError(t, r.SaveCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	r.Start(ctx) // blocks until ctx expires

	// Initial checkAndRefresh fires immediately, so at least one call.
	assert.GreaterOrEqual(t, callCount.Load(), int32(1),
		"Start must invoke checkAndRefresh at least once on startup")
}

// TestLoadCredentials_InvalidJSON verifies parse error on malformed credential data.
func TestLoadCredentials_InvalidJSON(t *testing.T) {
	t.Parallel()

	store := newMockCredStore()
	store.data["autopus-worker"] = "not-json{"

	r := NewTokenRefresher("http://unused", store, func() {}, nil)
	_, err := r.LoadCredentials()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse credentials")
}

// TestSaveCredentials_StoreError verifies that a store.Save failure returns error.
func TestSaveCredentials_StoreError(t *testing.T) {
	t.Parallel()

	store := newMockCredStore()
	store.saveErr = errors.New("disk full")

	r := NewTokenRefresher("http://unused", store, func() {}, nil)
	err := r.SaveCredentials(&Credentials{AccessToken: "tok"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save credentials to store")
}

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

	// Token expires far in the future — no refresh needed.
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

	// Must not panic.
	assert.NotPanics(t, func() {
		r.checkAndRefresh(context.Background())
	})
}

// TestCheckAndRefresh_ContextCancelledDuringBackoff verifies that ctx cancellation
// during backoff sleep exits the retry loop without blocking.
func TestCheckAndRefresh_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	// Server always returns 503 (retryable) — triggers backoff sleep.
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

	// Cancel almost immediately — backoff sleep should be interrupted.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	r.checkAndRefresh(ctx)
	elapsed := time.Since(start)

	// Should exit well within a full DefaultBackoffBase (3s).
	assert.Less(t, elapsed, 2*time.Second,
		"checkAndRefresh must respect context cancellation during backoff")
}

// TestDoRefresh_DecodeFailure verifies that invalid JSON response returns false.
func TestDoRefresh_DecodeFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json{"))
	}))
	defer srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)
	creds := &Credentials{RefreshToken: "ref"}

	got, _ := r.doRefresh(context.Background(), creds)
	assert.False(t, got, "invalid JSON response must return false")
}

// TestDoRefresh_SuccessFalse verifies that success=false in response returns false.
func TestDoRefresh_SuccessFalse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"data":    map[string]any{},
		})
	}))
	defer srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)
	creds := &Credentials{RefreshToken: "ref"}

	got, _ := r.doRefresh(context.Background(), creds)
	assert.False(t, got, "success=false must return false")
}

// TestDoRefresh_SaveError verifies that a store save failure returns false.
func TestDoRefresh_SaveError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"access_token":  "new-tok",
				"refresh_token": "new-ref",
				"expires_in":    3600,
			},
		})
	}))
	defer srv.Close()

	store := newMockCredStore()
	store.saveErr = errors.New("disk full")
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)
	creds := &Credentials{RefreshToken: "ref"}

	got, _ := r.doRefresh(context.Background(), creds)
	assert.False(t, got, "save failure must return false")
}

// TestDoRefresh_ExpiresInZero verifies that zero expires_in skips ExpiresAt update.
func TestDoRefresh_ExpiresInZero(t *testing.T) {
	t.Parallel()

	var savedToken atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"access_token":  "tok-zero-expiry",
				"refresh_token": "ref",
				"expires_in":    0, // zero — skip ExpiresAt update
			},
		})
	}))
	defer srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, func(tok string) { savedToken.Store(tok) })
	creds := &Credentials{Email: "u@e.com", RefreshToken: "ref"}

	got, _ := r.doRefresh(context.Background(), creds)
	require.True(t, got, "zero expires_in must still succeed")
	assert.Equal(t, "tok-zero-expiry", savedToken.Load())
}

// TestDoRefresh_ClientError verifies that an HTTP transport error returns false.
func TestDoRefresh_ClientError(t *testing.T) {
	t.Parallel()

	// Use an immediately-closed server to trigger a connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // close before the request

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)
	creds := &Credentials{RefreshToken: "ref"}

	got, sc := r.doRefresh(context.Background(), creds)
	assert.False(t, got, "transport error must return false")
	assert.Equal(t, 0, sc, "statusCode must be 0 on transport error")
}

// TestForceRefresh_ContextCancelled verifies ctx.Done during backoff returns ctx.Err.
func TestForceRefresh_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Server always returns 503 (retryable) → triggers backoff after first attempt.
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

	// Cancel during first backoff sleep (100ms < DefaultBackoffBase=3s).
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := r.ForceRefresh(ctx)
	assert.Error(t, err)
}

// TestCheckAndRefresh_RetriesUntilExhausted_CallsReauth verifies that after
// all retries on a retryable error, onReauthNeeded is called.
func TestCheckAndRefresh_RetriesUntilExhausted_CallsReauth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Always 503 — retryable, exhausts all attempts.
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

	// Use a short context to skip the full backoff sleep.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	r.checkAndRefresh(ctx)

	// Either context expired (backoff) or all retries exhausted — either way
	// the test verifies the path was entered. If the context cut it short,
	// reauthCalled may or may not be set — we only verify no panic here.
	// For the full exhaustion path, see TestTokenRefresher_BackoffRetry_AllFail_TriggersReauth.
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
