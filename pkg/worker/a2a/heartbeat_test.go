package a2a

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeartbeat_SendsAtInterval(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	hb := NewHeartbeat(func() error {
		count.Add(1)
		return nil
	}, func() {})
	hb.interval = 10 * time.Millisecond
	hb.timeout = 200 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	hb.Start(ctx)

	time.Sleep(55 * time.Millisecond)
	cancel()
	time.Sleep(5 * time.Millisecond)

	got := count.Load()
	assert.GreaterOrEqual(t, got, int32(3), "expected at least 3 sends in 55ms with 10ms interval")
}

func TestHeartbeat_AckResetsTimeout(t *testing.T) {
	t.Parallel()

	var timedOut atomic.Bool
	hb := NewHeartbeat(func() error { return nil }, func() {
		timedOut.Store(true)
	})
	hb.interval = 10 * time.Millisecond
	hb.timeout = 30 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hb.Start(ctx)

	// Keep acking to prevent timeout.
	for i := 0; i < 5; i++ {
		time.Sleep(15 * time.Millisecond)
		hb.Ack()
	}

	assert.False(t, timedOut.Load(), "should not timeout when Ack is called regularly")
}

func TestHeartbeat_TimeoutFires(t *testing.T) {
	t.Parallel()

	var timedOut atomic.Bool
	hb := NewHeartbeat(func() error { return nil }, func() {
		timedOut.Store(true)
	})
	hb.interval = 5 * time.Millisecond
	hb.timeout = 20 * time.Millisecond
	// Set lastAck far in the past so timeout triggers immediately.
	hb.lastAck = time.Now().Add(-1 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hb.Start(ctx)

	require.Eventually(t, func() bool {
		return timedOut.Load()
	}, 100*time.Millisecond, 2*time.Millisecond, "timeout callback should fire")
}

func TestHeartbeat_ContextCancellation(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	hb := NewHeartbeat(func() error {
		count.Add(1)
		return nil
	}, func() {})
	hb.interval = 10 * time.Millisecond
	hb.timeout = 1 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	hb.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()
	// Give the goroutine time to observe the cancellation.
	time.Sleep(20 * time.Millisecond)
	snapshot := count.Load()
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, snapshot, count.Load(), "no more sends after context cancelled")
}

// --- S1 (worker side): Heartbeat sends agent/heartbeat JSON-RPC every 30s ---
// Tests below reference NewHeartbeatWithJSONRPC and HeartbeatMessage — not yet implemented.

// TestHeartbeat_SendsJSONRPCHeartbeat asserts S1 (worker side):
// The heartbeat sendFn should transmit a valid agent/heartbeat JSON-RPC 2.0 message.
// The backend responds with {"status":"ok"} and Ack() is called automatically.
func TestHeartbeat_SendsJSONRPCHeartbeat(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var sentMessages [][]byte
	sendFn := func(msg []byte) error {
		cp := make([]byte, len(msg))
		copy(cp, msg)
		mu.Lock()
		sentMessages = append(sentMessages, cp)
		mu.Unlock()
		return nil
	}

	hb := NewHeartbeatWithJSONRPC(sendFn, func() {})
	hb.interval = 10 * time.Millisecond
	hb.timeout = 200 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	hb.Start(ctx)
	time.Sleep(25 * time.Millisecond)
	cancel()
	time.Sleep(5 * time.Millisecond)

	mu.Lock()
	msgs := make([][]byte, len(sentMessages))
	copy(msgs, sentMessages)
	mu.Unlock()

	// Then at least one message was sent
	require.NotEmpty(t, msgs, "heartbeat should send at least one message")

	// And the message is a valid agent/heartbeat JSON-RPC request
	var req JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &req))
	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, MethodHeartbeat, req.Method, "heartbeat must use agent/heartbeat method (S1)")
	assert.NotEmpty(t, req.ID, "heartbeat request must have an ID")
}

// TestHeartbeat_BackendResponseOK_CallsAck asserts S1 (worker side):
// When the backend responds with {"status":"ok"} to an agent/heartbeat,
// the Ack() is called, updating lastAck and preventing timeout.
func TestHeartbeat_BackendResponseOK_CallsAck(t *testing.T) {
	t.Parallel()

	var timedOut atomic.Bool
	hb := NewHeartbeat(func() error { return nil }, func() { timedOut.Store(true) })
	hb.interval = 10 * time.Millisecond
	hb.timeout = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hb.Start(ctx)

	// Simulate backend sending {"status":"ok"} response — worker calls HandleHeartbeatResponse
	for i := 0; i < 3; i++ {
		time.Sleep(12 * time.Millisecond)
		// HandleHeartbeatResponse does not exist yet — compile error
		hb.HandleHeartbeatResponse(map[string]string{"status": "ok"}) // method does not exist yet
	}

	// Then no timeout should have occurred
	assert.False(t, timedOut.Load(), "heartbeat ack from backend should prevent timeout (S1)")
}
