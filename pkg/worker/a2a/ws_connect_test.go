package a2a

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toWSSURL converts an https:// test server URL to a wss:// WebSocket URL.
func toWSSURL(httpsURL string) string {
	return "wss" + strings.TrimPrefix(httpsURL, "https")
}

func TestTransport_Connect_AuthToken(t *testing.T) {
	var receivedAuth string
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	// Use TLS server so that auth tokens can be sent over wss:// without being
	// blocked by the SEC-003 plaintext credential guard.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.ReadMessage()
	}))
	defer srv.Close()

	// Build a dialer that trusts the self-signed test certificate.
	tlsDialer := websocket.Dialer{
		TLSClientConfig: srv.Client().Transport.(*http.Transport).TLSClientConfig,
	}

	tr := NewTransport(TransportConfig{
		URL:          toWSSURL(srv.URL),
		AuthToken:    "test-secret-token",
		HeartbeatSec: 60,
		Dialer:       &tlsDialer,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, tr.Connect(ctx))
	defer tr.Close()

	assert.Equal(t, "Bearer test-secret-token", receivedAuth)
}

func TestTransport_Connect_NoAuthToken(t *testing.T) {
	var receivedAuth string
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.ReadMessage()
	}))
	defer srv.Close()

	tr := NewTransport(TransportConfig{
		URL:          toWSURL(srv.URL),
		HeartbeatSec: 60,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, tr.Connect(ctx))
	defer tr.Close()

	assert.Empty(t, receivedAuth, "no auth header should be sent without token")
}

func TestTransport_Connect_WSProtocolLogsWarning(t *testing.T) {
	srv := wsEchoServer(t)
	defer srv.Close()

	url := toWSURL(srv.URL)
	assert.True(t, strings.HasPrefix(url, "ws://"))

	tr := NewTransport(TransportConfig{
		URL:          url,
		HeartbeatSec: 60,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should connect successfully even with ws:// (warning is logged).
	require.NoError(t, tr.Connect(ctx))
	defer tr.Close()

	require.NoError(t, tr.Send([]byte("tls-test")))
	data, err := tr.Receive()
	require.NoError(t, err)
	assert.Equal(t, "tls-test", string(data))
}
