package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// @AX:NOTE [AUTO] magic constants — heartbeat SLA: 30s interval, 60s timeout; changing either alters reconnect sensitivity
const (
	defaultHeartbeatInterval = 30 * time.Second
	defaultHeartbeatTimeout  = 60 * time.Second
)

// Heartbeat sends periodic heartbeat messages over the A2A WebSocket
// and detects connection loss via timeout.
type Heartbeat struct {
	interval  time.Duration
	timeout   time.Duration
	sendFn    func() error
	onTimeout func()
	mu        sync.Mutex
	lastAck   time.Time
}

// NewHeartbeat creates a Heartbeat that calls sendFn every 30s.
// If no Ack is received within 60s, onTimeout is called.
func NewHeartbeat(sendFn func() error, onTimeout func()) *Heartbeat {
	return &Heartbeat{
		interval:  defaultHeartbeatInterval,
		timeout:   defaultHeartbeatTimeout,
		sendFn:    sendFn,
		onTimeout: onTimeout,
		lastAck:   time.Now(),
	}
}

// Start runs the heartbeat loop in a goroutine until ctx is cancelled.
func (h *Heartbeat) Start(ctx context.Context) {
	go h.run(ctx)
}

func (h *Heartbeat) run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.tick()
		}
	}
}

func (h *Heartbeat) tick() {
	h.mu.Lock()
	elapsed := time.Since(h.lastAck)
	h.mu.Unlock()

	if elapsed >= h.timeout {
		h.onTimeout()
		return
	}

	// Best-effort send; timeout check handles failures.
	_ = h.sendFn()
}

// Ack records that a heartbeat response was received.
func (h *Heartbeat) Ack() {
	h.mu.Lock()
	h.lastAck = time.Now()
	h.mu.Unlock()
}

// @AX:ANCHOR [AUTO] protocol entry point — sole constructor for JSON-RPC heartbeat; server.go wires this at startup — fan_in: 2
// NewHeartbeatWithJSONRPC creates a Heartbeat that sends agent/heartbeat JSON-RPC 2.0
// messages instead of raw WebSocket ping frames (S1 requirement).
func NewHeartbeatWithJSONRPC(sendFn func(msg []byte) error, onTimeout func()) *Heartbeat {
	return &Heartbeat{
		interval:  defaultHeartbeatInterval,
		timeout:   defaultHeartbeatTimeout,
		onTimeout: onTimeout,
		lastAck:   time.Now(),
		sendFn: func() error {
			id := fmt.Sprintf("hb-%d", time.Now().UnixNano())
			req := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"` + id + `"`),
				Method:  MethodHeartbeat,
				Params:  json.RawMessage(`{}`),
			}
			data, err := json.Marshal(req)
			if err != nil {
				return fmt.Errorf("heartbeat marshal: %w", err)
			}
			return sendFn(data)
		},
	}
}

// @AX:TODO [AUTO] @AX:CYCLE:1 no dedicated unit test for HandleHeartbeatResponse status != "ok" branch
// HandleHeartbeatResponse records an acknowledgement from a backend heartbeat response.
// Called when the backend sends {"status":"ok"} in reply to an agent/heartbeat request.
func (h *Heartbeat) HandleHeartbeatResponse(result map[string]string) {
	if result["status"] == "ok" {
		h.Ack()
	}
}
