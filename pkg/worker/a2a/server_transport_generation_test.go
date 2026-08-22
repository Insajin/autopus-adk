package a2a

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTransportOnlyServer wires a connected transport without Start's message
// loop. The loop is itself a reconnect driver, so a test that measures how many
// reconnects a connection loss produces cannot have it running.
func newTransportOnlyServer(t *testing.T, ctx context.Context, mb *mockBackend) *Server {
	t.Helper()
	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "generation-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})
	srv.transport = NewTransport(TransportConfig{
		URL:              toWebSocketURL(srv.config.BackendURL) + "/ws/a2a",
		HeartbeatSec:     30,
		ReconnectBaseSec: 3,
		ReconnectFactor:  2,
		MaxRetries:       4,
	})
	require.NoError(t, srv.transport.Connect(ctx))
	t.Cleanup(func() { _ = srv.transport.Close() })
	return srv
}

// assertNoFurtherMessage fails if the backend receives anything during a quiet
// window. waitForMessages cannot express this: it fatals on the timeout it needs.
func assertNoFurtherMessage(t *testing.T, mb *mockBackend, quiet time.Duration) {
	t.Helper()
	select {
	case extra := <-mb.msgCh:
		t.Fatalf("unexpected extra message from a duplicate reconnect: %s", extra)
	case <-time.After(quiet):
	}
}

// TestServer_ReconnectTransportFrom_CoalescesDriversOfOneConnectionLoss pins the
// invariant the receive loop and the heartbeat timeout depend on: every driver
// that observed the same generation is answered by a single reconnect.
//
// Before the generation existed, reconnectMu only serialized them, so each
// driver in turn tore down the connection its predecessor had just established
// and re-registered the card again — observed in CI as
// `re-register agent card: ws write: websocket: close sent` racing
// `dial: connection refused` in TestServer_ReconnectTransport_ReRegistersAgentCard.
func TestServer_ReconnectTransportFrom_CoalescesDriversOfOneConnectionLoss(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv := newTransportOnlyServer(t, ctx, mb)

	generation := srv.transportGeneration()
	mb.closeConn()

	const drivers = 8
	errs := make([]error, drivers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for index := range drivers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[index] = srv.reconnectTransportFrom(ctx, generation)
		}()
	}
	close(start)
	wg.Wait()

	for index, err := range errs {
		require.NoError(t, err, "driver %d", index)
	}

	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var registration JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &registration))
	assert.Equal(t, MethodRegisterCard, registration.Method)
	assertNoFurtherMessage(t, mb, 300*time.Millisecond)

	assert.Equal(t, generation+1, srv.transportGeneration(),
		"one connection loss must advance the generation exactly once")
}

// TestServer_ReconnectTransportFrom_SkipsAConnectionAlreadyReplaced covers the
// late driver: a receive loop that was reading generation N and lost the race
// must not tear down the generation N+1 connection that replaced it.
func TestServer_ReconnectTransportFrom_SkipsAConnectionAlreadyReplaced(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv := newTransportOnlyServer(t, ctx, mb)

	stale := srv.transportGeneration()
	mb.closeConn()
	require.NoError(t, srv.reconnectTransportFrom(ctx, stale))
	mb.waitForMessages(t, 1, 5*time.Second)
	replaced := srv.transportGeneration()

	require.NoError(t, srv.reconnectTransportFrom(ctx, stale),
		"a stale generation is a no-op, not a failure")

	assertNoFurtherMessage(t, mb, 300*time.Millisecond)
	assert.Equal(t, replaced, srv.transportGeneration(),
		"a stale driver must not advance the generation")
}

// TestServer_ReconnectTransport_IsUnconditionalForExternalCallers keeps the
// public entry point usable for auth-token rotation: pkg/worker/auth calls
// SetAuthToken and then ReconnectTransport specifically to force a new
// connection, so it must not be skipped just because one already succeeded.
func TestServer_ReconnectTransport_IsUnconditionalForExternalCallers(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv := newTransportOnlyServer(t, ctx, mb)

	mb.closeConn()
	require.NoError(t, srv.ReconnectTransport(ctx))
	first := srv.transportGeneration()
	mb.waitForMessages(t, 1, 5*time.Second)

	require.NoError(t, srv.ReconnectTransport(ctx))
	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var registration JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &registration))
	assert.Equal(t, MethodRegisterCard, registration.Method)
	assert.Equal(t, first+1, srv.transportGeneration())
}
