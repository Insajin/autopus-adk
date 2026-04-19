package a2a

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDispatchTask_TimeoutSec_EnforcesDeadline verifies that SecurityPolicy.TimeoutSec
// is applied as a hard deadline via context.WithTimeout. The subprocess (handler) should
// be interrupted when the deadline expires.
func TestDispatchTask_TimeoutSec_EnforcesDeadline(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handlerDone := make(chan error, 1)
	handler := func(ctx context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
		// Simulate a long-running task that respects ctx cancellation.
		select {
		case <-ctx.Done():
			handlerDone <- ctx.Err()
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
			handlerDone <- nil
			return &TaskResult{Status: StatusCompleted}, nil
		}
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"test"},
		Handler:    handler,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	// Send task with TimeoutSec = 2 (short timeout to trigger deadline).
	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "task-timeout",
			Payload:        json.RawMessage(`{}`),
			SecurityPolicy: SecurityPolicy{TimeoutSec: 2},
		}),
	}
	data, _ := json.Marshal(taskReq)
	require.NoError(t, mb.sendMessage(data))

	// Handler should be interrupted within ~2 seconds by the deadline.
	select {
	case err := <-handlerDone:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("handler was not interrupted by TimeoutSec deadline")
	}
}

// TestDispatchTask_ZeroTimeoutSec_NoBehaviorChange verifies that TimeoutSec = 0
// does not apply any timeout (backward compatibility).
func TestDispatchTask_ZeroTimeoutSec_NoBehaviorChange(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handlerDone := make(chan struct{})
	handler := func(ctx context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
		close(handlerDone)
		return &TaskResult{Status: StatusCompleted}, nil
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"test"},
		Handler:    handler,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	// Send task with TimeoutSec = 0 (no timeout enforcement).
	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "task-no-timeout",
			Payload:        json.RawMessage(`{}`),
			SecurityPolicy: SecurityPolicy{TimeoutSec: 0},
		}),
	}
	data, _ := json.Marshal(taskReq)
	require.NoError(t, mb.sendMessage(data))

	// Handler should complete normally without timeout interference.
	select {
	case <-handlerDone:
		// Success: handler completed without timeout.
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not complete in time")
	}
}
