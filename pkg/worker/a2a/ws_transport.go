package a2a

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// maxMessageSize limits the maximum incoming WebSocket message to 10 MB (SEC-004).
const maxMessageSize = 10 * 1024 * 1024

// TransportConfig holds WebSocket transport settings.
type TransportConfig struct {
	URL              string
	AuthToken        string // Bearer token for WebSocket auth (SEC-005)
	HeartbeatSec     int
	ReconnectBaseSec int
	ReconnectFactor  int
	MaxRetries       int
	// Dialer overrides the default WebSocket dialer (used in tests for TLS certs).
	Dialer *websocket.Dialer
}

// Transport wraps a gorilla/websocket connection with heartbeat and reconnect.
type Transport struct {
	config TransportConfig
	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
	cancel context.CancelFunc
}

// NewTransport creates a new WebSocket transport.
func NewTransport(config TransportConfig) *Transport {
	if config.HeartbeatSec <= 0 {
		config.HeartbeatSec = 30
	}
	if config.ReconnectBaseSec <= 0 {
		config.ReconnectBaseSec = 3
	}
	if config.ReconnectFactor <= 0 {
		config.ReconnectFactor = 2
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 4
	}
	return &Transport{config: config}
}

// Connect dials the WebSocket endpoint and starts the heartbeat loop.
func (t *Transport) Connect(ctx context.Context) error {
	// SEC-003/V5: Block sending credentials over unencrypted WebSocket.
	// Plaintext ws:// would expose API keys and JWT tokens on the wire.
	if strings.HasPrefix(t.config.URL, "ws://") {
		if t.config.AuthToken != "" {
			return fmt.Errorf("refusing to send auth token over unencrypted ws:// — use wss://")
		}
		log.Printf("[a2a] WARNING: unencrypted WebSocket connection — use wss:// in production")
	}

	// SEC-005: Attach auth token if configured.
	var header http.Header
	if t.config.AuthToken != "" {
		header = http.Header{"Authorization": []string{"Bearer " + t.config.AuthToken}}
	}

	dialer := websocket.DefaultDialer
	if t.config.Dialer != nil {
		dialer = t.config.Dialer
	}
	conn, _, err := dialer.DialContext(ctx, t.config.URL, header)
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", t.config.URL, err)
	}

	// SEC-004: Limit incoming message size.
	conn.SetReadLimit(maxMessageSize)

	t.mu.Lock()
	t.conn = conn
	t.closed = false
	t.mu.Unlock()

	// Set pong handler to extend read deadline.
	pongWait := time.Duration(t.config.HeartbeatSec*2) * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("set initial read deadline: %w", err)
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	var hbCtx context.Context
	hbCtx, t.cancel = context.WithCancel(ctx)
	go t.startHeartbeat(hbCtx)

	log.Printf("[a2a] connected to %s", t.config.URL)
	return nil
}

// Send writes a message to the WebSocket connection.
func (t *Transport) Send(msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return fmt.Errorf("ws not connected")
	}
	if err := t.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return fmt.Errorf("ws write: %w", err)
	}
	return nil
}

// Receive reads the next message from the WebSocket connection.
func (t *Transport) Receive() ([]byte, error) {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("ws not connected")
	}

	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("ws read: %w", err)
	}
	return data, nil
}

// Close gracefully shuts down the transport.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	if t.cancel != nil {
		t.cancel()
	}
	if t.conn == nil {
		return nil
	}

	// Send close frame.
	deadline := time.Now().Add(3 * time.Second)
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = t.conn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
	return t.conn.Close()
}

// SetAuthToken updates the bearer token used for future connects/reconnects.
func (t *Transport) SetAuthToken(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config.AuthToken = token
}

// startHeartbeat sends ping frames at the configured interval.
func (t *Transport) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(t.config.HeartbeatSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.mu.Lock()
			if t.conn == nil || t.closed {
				t.mu.Unlock()
				return
			}
			deadline := time.Now().Add(5 * time.Second)
			err := t.conn.WriteControl(websocket.PingMessage, nil, deadline)
			t.mu.Unlock()

			if err != nil {
				log.Printf("[a2a] heartbeat ping failed: %v", err)
				return
			}
		}
	}
}

// Reconnect attempts to re-establish the WebSocket connection with exponential backoff.
// base=3s, factor=2, max retries=4.
func (t *Transport) Reconnect(ctx context.Context) error {
	t.mu.Lock()
	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()

	base := float64(t.config.ReconnectBaseSec)
	factor := float64(t.config.ReconnectFactor)

	for attempt := 0; attempt < t.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(base*math.Pow(factor, float64(attempt-1))) * time.Second
			log.Printf("[a2a] reconnect attempt %d/%d in %v", attempt+1, t.config.MaxRetries, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := t.Connect(ctx); err != nil {
			log.Printf("[a2a] reconnect attempt %d failed: %v", attempt+1, err)
			continue
		}
		log.Printf("[a2a] reconnected on attempt %d", attempt+1)
		return nil
	}

	return fmt.Errorf("ws reconnect failed after %d attempts", t.config.MaxRetries)
}
