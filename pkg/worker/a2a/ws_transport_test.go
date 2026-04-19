package a2a

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wsEchoServer creates a test WebSocket server that echoes messages back.
func wsEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, msg)
		}
	}))
}

func toWSURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func TestNewTransport_Defaults(t *testing.T) {
	t.Parallel()

	// All zero values should be replaced with defaults.
	tr := NewTransport(TransportConfig{URL: "ws://localhost/test"})
	assert.Equal(t, 30, tr.config.HeartbeatSec)
	assert.Equal(t, 3, tr.config.ReconnectBaseSec)
	assert.Equal(t, 2, tr.config.ReconnectFactor)
	assert.Equal(t, 4, tr.config.MaxRetries)
}

func TestNewTransport_CustomValues(t *testing.T) {
	t.Parallel()

	tr := NewTransport(TransportConfig{
		URL:              "ws://localhost/test",
		HeartbeatSec:     10,
		ReconnectBaseSec: 1,
		ReconnectFactor:  3,
		MaxRetries:       2,
	})
	assert.Equal(t, 10, tr.config.HeartbeatSec)
	assert.Equal(t, 1, tr.config.ReconnectBaseSec)
	assert.Equal(t, 3, tr.config.ReconnectFactor)
	assert.Equal(t, 2, tr.config.MaxRetries)
}

func TestTransport_ConnectAndSendReceive(t *testing.T) {
	srv := wsEchoServer(t)
	defer srv.Close()

	tr := NewTransport(TransportConfig{
		URL:          toWSURL(srv.URL),
		HeartbeatSec: 60, // Long interval to avoid interference.
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, tr.Connect(ctx))
	defer func() {
		require.NoError(t, tr.Close())
	}()

	// Send and receive echo.
	require.NoError(t, tr.Send([]byte("hello")))
	data, err := tr.Receive()
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestTransport_Send_NotConnected(t *testing.T) {
	t.Parallel()

	tr := NewTransport(TransportConfig{URL: "ws://localhost/nope"})
	err := tr.Send([]byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestTransport_Receive_NotConnected(t *testing.T) {
	t.Parallel()

	tr := NewTransport(TransportConfig{URL: "ws://localhost/nope"})
	_, err := tr.Receive()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestTransport_Close_Idempotent(t *testing.T) {
	t.Parallel()

	// Closing a transport that was never connected should not error.
	tr := NewTransport(TransportConfig{URL: "ws://localhost/nope"})
	require.NoError(t, tr.Close())
	require.NoError(t, tr.Close())
}

func TestTransport_Reconnect_Success(t *testing.T) {
	srv := wsEchoServer(t)
	defer srv.Close()

	tr := NewTransport(TransportConfig{
		URL:              toWSURL(srv.URL),
		HeartbeatSec:     60,
		ReconnectBaseSec: 1,
		ReconnectFactor:  2,
		MaxRetries:       3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initial connect.
	require.NoError(t, tr.Connect(ctx))

	// Reconnect should close old connection and establish a new one.
	require.NoError(t, tr.Reconnect(ctx))
	defer func() {
		require.NoError(t, tr.Close())
	}()

	// Verify the new connection works.
	require.NoError(t, tr.Send([]byte("after-reconnect")))
	data, err := tr.Receive()
	require.NoError(t, err)
	assert.Equal(t, "after-reconnect", string(data))
}

func TestTransport_Reconnect_AllRetriesFail(t *testing.T) {
	tr := NewTransport(TransportConfig{
		URL:              "ws://127.0.0.1:1/invalid", // Nothing listening.
		HeartbeatSec:     60,
		ReconnectBaseSec: 1,
		ReconnectFactor:  1, // No backoff growth to keep test fast.
		MaxRetries:       2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := tr.Reconnect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reconnect failed")
}

func TestTransport_Reconnect_ContextCanceled(t *testing.T) {
	tr := NewTransport(TransportConfig{
		URL:              "ws://127.0.0.1:1/invalid",
		HeartbeatSec:     60,
		ReconnectBaseSec: 5, // Long delay so we can cancel during it.
		ReconnectFactor:  2,
		MaxRetries:       4,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := tr.Reconnect(ctx)
	require.Error(t, err)
	// Should hit context deadline before completing retries.
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTransport_Heartbeat_SendsPing(t *testing.T) {
	var pingCount atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		conn.SetPingHandler(func(string) error {
			pingCount.Add(1)
			return conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
		})
		// Keep reading to process control frames.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	tr := NewTransport(TransportConfig{
		URL:          toWSURL(srv.URL),
		HeartbeatSec: 1, // 1-second heartbeat for fast test.
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, tr.Connect(ctx))
	defer func() {
		require.NoError(t, tr.Close())
	}()

	// Wait enough time for at least 2 pings.
	time.Sleep(2500 * time.Millisecond)
	assert.GreaterOrEqual(t, pingCount.Load(), int32(2), "expected at least 2 pings")
}
