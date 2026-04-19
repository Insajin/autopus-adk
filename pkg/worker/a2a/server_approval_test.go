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

func TestServer_HandleApproval_CallsCallbackAndUpdatesStatus(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	var (
		callbackMu     sync.Mutex
		callbackCalled bool
		callbackParams ApprovalRequestParams
	)

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
		ApprovalCallback: func(params ApprovalRequestParams) {
			callbackMu.Lock()
			defer callbackMu.Unlock()
			callbackCalled = true
			callbackParams = params
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))

	// Consume registration message.
	mb.waitForMessages(t, 1, 3*time.Second)

	// Pre-create the task so UpdateTaskStatus finds it.
	srv.mu.Lock()
	srv.tasks["task-approval-001"] = &Task{ID: "task-approval-001", Status: StatusWorking}
	srv.mu.Unlock()

	// Send an approval request.
	approvalReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  MethodApproval,
		Params: mustMarshal(ApprovalRequestParams{
			TaskID:    "task-approval-001",
			Action:    "rm -rf /tmp/build",
			RiskLevel: "high",
			Context:   "Deleting build directory",
		}),
	}
	data, err := json.Marshal(approvalReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	// Expect a status update notification (input-required).
	msgs := mb.waitForMessages(t, 1, 5*time.Second)

	var statusNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[0], &statusNotif))
	assert.Equal(t, MethodStatusUpdate, statusNotif.Method)

	// Verify the status update contains input-required.
	paramsBytes, err := json.Marshal(statusNotif.Params)
	require.NoError(t, err)
	var statusParams StatusUpdateParams
	require.NoError(t, json.Unmarshal(paramsBytes, &statusParams))
	assert.Equal(t, StatusInputRequired, statusParams.Status)
	assert.Equal(t, "task-approval-001", statusParams.TaskID)

	// Verify callback was invoked with correct params.
	callbackMu.Lock()
	defer callbackMu.Unlock()
	assert.True(t, callbackCalled)
	assert.Equal(t, "task-approval-001", callbackParams.TaskID)
	assert.Equal(t, "rm -rf /tmp/build", callbackParams.Action)
	assert.Equal(t, "high", callbackParams.RiskLevel)
	assert.Equal(t, "Deleting build directory", callbackParams.Context)
}

func TestServer_SendApprovalResponse_SendsCorrectNotification(t *testing.T) {
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

	// Consume registration message.
	mb.waitForMessages(t, 1, 3*time.Second)

	// Pre-create the task.
	srv.mu.Lock()
	srv.tasks["task-resp-001"] = &Task{ID: "task-resp-001", Status: StatusInputRequired}
	srv.mu.Unlock()

	// Send approval response.
	err := srv.SendApprovalResponse("task-resp-001", "approve")
	require.NoError(t, err)

	// Expect: working status update + approval response notification = 2 messages.
	msgs := mb.waitForMessages(t, 2, 5*time.Second)

	// First message: status update to working.
	var statusNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[0], &statusNotif))
	assert.Equal(t, MethodStatusUpdate, statusNotif.Method)

	// Second message: approval response notification.
	var respNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[1], &respNotif))
	assert.Equal(t, MethodApprovalResponse, respNotif.Method)

	respBytes, err := json.Marshal(respNotif.Params)
	require.NoError(t, err)
	var respParams ApprovalResponseParams
	require.NoError(t, json.Unmarshal(respBytes, &respParams))
	assert.Equal(t, "task-resp-001", respParams.TaskID)
	assert.Equal(t, "approve", respParams.Decision)
}
