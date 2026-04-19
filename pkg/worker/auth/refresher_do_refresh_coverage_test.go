package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				"expires_in":    0,
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	store := newMockCredStore()
	r := NewTokenRefresher(srv.URL, store, func() {}, nil)
	creds := &Credentials{RefreshToken: "ref"}

	got, sc := r.doRefresh(context.Background(), creds)
	assert.False(t, got, "transport error must return false")
	assert.Equal(t, 0, sc, "statusCode must be 0 on transport error")
}
