package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconnect_TransportError verifies that a ReconnectTransport error propagates.
func TestReconnect_TransportError(t *testing.T) {
	t.Parallel()

	fakeSrv := newRefreshServer(t, "ok-token")
	defer fakeSrv.Close()

	store := newCredStoreWithCreds("ref-tok")
	transportErr := errors.New("websocket dial failed")
	srv := &testServerReconnecter{reconnectErr: transportErr}
	refresher := NewTokenRefresher(fakeSrv.URL, store, func() {}, nil)
	r := NewReconnector(refresher, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.Reconnect(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, transportErr, "transport error must propagate from Reconnect")

	// SetAuthToken must have been called (refresh succeeded).
	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.setAuthCalls, 1, "SetAuthToken must be called even when reconnect fails")
	assert.Equal(t, 1, srv.reconnectCalls, "ReconnectTransport must have been attempted")
}
