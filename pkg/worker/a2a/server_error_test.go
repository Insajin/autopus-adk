package a2a

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_SendMessage_InvalidParams(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // registration

	// Send a task with invalid params (not valid SendMessageParams JSON).
	badReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  MethodSendMessage,
		Params:  json.RawMessage(`"not-an-object"`),
	}
	data, _ := json.Marshal(badReq)
	require.NoError(t, mb.sendMessage(data))

	// Should receive an error response with code -32602.
	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "invalid params")
}

func TestServer_CancelTask_InvalidParams(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // registration

	// Send cancel with invalid params.
	badCancel := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
		Method:  MethodCancelTask,
		Params:  json.RawMessage(`999`),
	}
	data, _ := json.Marshal(badCancel)
	require.NoError(t, mb.sendMessage(data))

	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestServer_UnknownMethod(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // registration

	// Send an unknown method — should be silently ignored (logged).
	unknownReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`12`),
		Method:  "unknown/method",
		Params:  json.RawMessage(`{}`),
	}
	data, _ := json.Marshal(unknownReq)
	require.NoError(t, mb.sendMessage(data))

	// Send a known method after to verify the server is still responsive.
	cancelReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`13`),
		Method:  MethodCancelTask,
		Params: mustMarshal(struct {
			TaskID string `json:"task_id"`
		}{TaskID: "nonexistent"}),
	}
	cancelData, _ := json.Marshal(cancelReq)
	require.NoError(t, mb.sendMessage(cancelData))

	// Should get cancel status + cancel response = 2.
	msgs := mb.waitForMessages(t, 2, 3*time.Second)
	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[1], &resp))
	assert.Nil(t, resp.Error)
}

func TestServer_SendMessage_DuplicateTaskID(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handler := func(_ context.Context, taskID string, _ json.RawMessage) (*TaskResult, error) {
		// Slow handler so the task stays in-flight.
		<-time.After(10 * time.Second)
		return &TaskResult{}, nil
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler:    handler,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // registration

	// Send first task.
	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`20`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "dup-task",
			Payload:        json.RawMessage(`{}`),
			SecurityPolicy: SecurityPolicy{},
		}),
	}
	data, _ := json.Marshal(taskReq)
	require.NoError(t, mb.sendMessage(data))

	// Wait for working status.
	mb.waitForMessages(t, 1, 3*time.Second)

	// Send duplicate task ID — should be rejected with error.
	dupReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`21`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "dup-task",
			Payload:        json.RawMessage(`{}`),
			SecurityPolicy: SecurityPolicy{},
		}),
	}
	dupData, _ := json.Marshal(dupReq)
	require.NoError(t, mb.sendMessage(dupData))

	// Should receive error response for the duplicate.
	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "duplicate task ID")
}

func TestServer_MessageLoop_BackoffOnReceiveError(t *testing.T) {
	mb := newMockBackend()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // registration

	// Close the backend connection to trigger receive errors in messageLoop.
	// This exercises the backoff path (consecutive errors → exponential sleep).
	mb.close()

	// Wait long enough for at least one backoff cycle (initial 500ms).
	time.Sleep(800 * time.Millisecond)

	// Cancel context — messageLoop should exit cleanly from the backoff select.
	cancel()

	// Give messageLoop time to exit.
	time.Sleep(200 * time.Millisecond)

	// Verify server can be closed without hanging.
	require.NoError(t, srv.Close())
}

func TestServer_MessageLoop_ReconnectsAfterReceiveError(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // initial registration

	mb.closeConn()

	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var regReq JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &regReq))
	assert.Equal(t, MethodRegisterCard, regReq.Method)
}
