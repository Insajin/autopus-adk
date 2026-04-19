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
