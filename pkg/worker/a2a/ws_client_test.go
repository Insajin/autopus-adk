package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_InitialState(t *testing.T) {
	t.Parallel()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{URL: "ws://localhost/test"},
		AgentCard: AgentCard{Name: "test"},
	})
	assert.Equal(t, StateDisconnected, client.State())
}

func TestClient_Connect_Success(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{URL: mb.wsURL() + "/ws/a2a", HeartbeatSec: 60},
		AgentCard: AgentCard{Name: "test-worker", Skills: []string{"coding"}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	defer func() { _ = client.Close() }()

	assert.Equal(t, StateConnected, client.State())

	// Verify agent card registration was sent.
	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var req JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &req))
	assert.Equal(t, MethodRegisterCard, req.Method)
}

func TestClient_Connect_TransportFailure(t *testing.T) {
	t.Parallel()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{URL: "ws://127.0.0.1:1/invalid", HeartbeatSec: 60},
		AgentCard: AgentCard{Name: "fail"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client connect")
	assert.Equal(t, StateDisconnected, client.State())
}

func TestClient_OnConnected_Fires(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{URL: mb.wsURL() + "/ws/a2a", HeartbeatSec: 60},
		AgentCard: AgentCard{Name: "cb-test"},
	})

	var called atomic.Bool
	client.OnConnected(func() { called.Store(true) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	defer func() { _ = client.Close() }()

	assert.True(t, called.Load(), "OnConnected should have been called")
}

func TestClient_OnDisconnected_Fires(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{
			URL:              mb.wsURL() + "/ws/a2a",
			HeartbeatSec:     60,
			ReconnectBaseSec: 1,
			ReconnectFactor:  1,
			MaxRetries:       1,
		},
		AgentCard: AgentCard{Name: "dc-test"},
	})

	var disconnectErr atomic.Value
	client.OnDisconnected(func(err error) { disconnectErr.Store(err) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	defer func() { _ = client.Close() }()

	// Close the backend to simulate connection loss, then handle it.
	mb.close()

	cause := errors.New("connection reset")
	// HandleConnectionLoss will try to reconnect which will fail.
	_ = client.HandleConnectionLoss(ctx, cause)

	stored := disconnectErr.Load()
	require.NotNil(t, stored)
	assert.Equal(t, cause, stored.(error))
}

func TestClient_OnReconnectFailed_Fires(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{
			URL:              mb.wsURL() + "/ws/a2a",
			HeartbeatSec:     60,
			ReconnectBaseSec: 1,
			ReconnectFactor:  1,
			MaxRetries:       1,
		},
		AgentCard: AgentCard{Name: "rf-test"},
	})

	var reconnectFailed atomic.Bool
	client.OnReconnectFailed(func(_ error) { reconnectFailed.Store(true) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	mb.close()

	err := client.HandleConnectionLoss(ctx, errors.New("lost"))
	require.Error(t, err)
	assert.True(t, reconnectFailed.Load(), "OnReconnectFailed should have been called")
}

// mockRecoverer implements StateRecoverer for testing state recovery.
type mockRecoverer struct {
	tasks     []Task
	recovered []Task
	mu        sync.Mutex
}

func (m *mockRecoverer) InFlightTasks() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks
}

func (m *mockRecoverer) OnStateRecovered(tasks []Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recovered = tasks
}

func TestClient_StateRecovery_OnReconnect(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	recoverer := &mockRecoverer{
		tasks: []Task{
			{ID: "task-1", Status: StatusWorking},
			{ID: "task-2", Status: StatusInputRequired},
		},
	}

	client := NewClient(ClientConfig{
		Transport: TransportConfig{
			URL:              mb.wsURL() + "/ws/a2a",
			HeartbeatSec:     60,
			ReconnectBaseSec: 1,
			ReconnectFactor:  1,
			MaxRetries:       2,
		},
		AgentCard:      AgentCard{Name: "recovery-test"},
		StateRecoverer: recoverer,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	defer func() { _ = client.Close() }()

	// Consume registration message.
	mb.waitForMessages(t, 1, 3*time.Second)

	// Trigger reconnection (transport will reconnect to the same server).
	err := client.HandleConnectionLoss(ctx, errors.New("test-loss"))
	require.NoError(t, err)
	assert.Equal(t, StateConnected, client.State())

	// Verify recovery callback was invoked.
	recoverer.mu.Lock()
	assert.Len(t, recoverer.recovered, 2)
	recoverer.mu.Unlock()
}

func TestClient_ConcurrentStateAccess(t *testing.T) {
	t.Parallel()

	client := NewClient(ClientConfig{
		Transport: TransportConfig{URL: "ws://localhost/test"},
		AgentCard: AgentCard{Name: "race-test"},
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.State()
		}()
	}
	wg.Wait()
}
