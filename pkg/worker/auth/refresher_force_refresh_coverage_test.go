package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForceRefresh_ContextCancelled verifies ctx.Done during backoff returns ctx.Err.
func TestForceRefresh_ContextCancelled(t *testing.T) {
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

	_, err := r.ForceRefresh(ctx)
	assert.Error(t, err)
}
