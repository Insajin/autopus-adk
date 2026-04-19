package a2a

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_CancelTask(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handler := func(ctx context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
		// Long-running handler that should be preempted.
		<-time.After(30 * time.Second)
		return &TaskResult{}, nil
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"slow"},
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

	sendReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "task-cancel",
			Payload:        json.RawMessage(`{}`),
			SecurityPolicy: SecurityPolicy{},
		}),
	}
	sendData, _ := json.Marshal(sendReq)
	require.NoError(t, mb.sendMessage(sendData))

	mb.waitForMessages(t, 1, 3*time.Second)

	cancelReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  MethodCancelTask,
		Params: mustMarshal(struct {
			TaskID string `json:"task_id"`
		}{TaskID: "task-cancel"}),
	}
	cancelData, _ := json.Marshal(cancelReq)
	require.NoError(t, mb.sendMessage(cancelData))

	msgs := mb.waitForMessages(t, 2, 5*time.Second)

	var statusNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[0], &statusNotif))
	assert.Equal(t, MethodStatusUpdate, statusNotif.Method)

	var cancelResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[1], &cancelResp))
	assert.Nil(t, cancelResp.Error)
}

func TestServer_HandlePolledTask(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handler := func(_ context.Context, taskID string, payload json.RawMessage) (*TaskResult, error) {
		return &TaskResult{
			Artifacts: []Artifact{{
				Name: "result.txt",
				Data: "handled " + taskID + ":" + string(payload),
			}},
		}, nil
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"poll"},
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

	err := srv.HandlePolledTask(ctx, PollResult{
		ID:      "poll-task-001",
		Payload: json.RawMessage(`{"prompt":"from poll"}`),
	})
	require.NoError(t, err)

	msgs := mb.waitForMessages(t, 2, 5*time.Second)

	var workingNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[0], &workingNotif))
	assert.Equal(t, MethodStatusUpdate, workingNotif.Method)

	var completedNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[1], &completedNotif))
	assert.Equal(t, MethodStatusUpdate, completedNotif.Method)
}

func TestServer_HandlePolledTask_InjectsModelIntoPayload(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	modelSeen := make(chan string, 1)
	handler := func(_ context.Context, _ string, payload json.RawMessage) (*TaskResult, error) {
		var msg struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		modelSeen <- msg.Model
		return &TaskResult{}, nil
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"poll"},
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

	err := srv.HandlePolledTask(ctx, PollResult{
		ID:      "poll-task-002",
		Model:   "gpt-5.4",
		Payload: json.RawMessage(`{"prompt":"from poll"}`),
	})
	require.NoError(t, err)

	select {
	case got := <-modelSeen:
		assert.Equal(t, "gpt-5.4", got)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive injected model")
	}
}
