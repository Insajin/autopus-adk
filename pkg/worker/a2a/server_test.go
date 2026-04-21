package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend simulates the backend WebSocket server for testing.
type mockBackend struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	conn     *websocket.Conn
	mu       sync.Mutex
	messages [][]byte
	msgCh    chan []byte
}

func newMockBackend() *mockBackend {
	mb := &mockBackend{
		messages: make([][]byte, 0),
		msgCh:    make(chan []byte, 32),
	}
	mb.upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/a2a", mb.handleWS)
	mb.server = httptest.NewServer(mux)
	return mb
}

func (mb *mockBackend) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := mb.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	mb.mu.Lock()
	mb.conn = conn
	mb.mu.Unlock()

	// Read loop: capture all messages from the client.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		mb.mu.Lock()
		mb.messages = append(mb.messages, data)
		mb.mu.Unlock()
		mb.msgCh <- data
	}
}

func (mb *mockBackend) sendMessage(msg []byte) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if mb.conn == nil {
		return fmt.Errorf("no client connected")
	}
	return mb.conn.WriteMessage(websocket.TextMessage, msg)
}

func (mb *mockBackend) closeConn() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if mb.conn != nil {
		_ = mb.conn.Close()
		mb.conn = nil
	}
}

func (mb *mockBackend) wsURL() string {
	return "ws" + strings.TrimPrefix(mb.server.URL, "http")
}

func (mb *mockBackend) close() {
	mb.mu.Lock()
	if mb.conn != nil {
		_ = mb.conn.Close()
	}
	mb.mu.Unlock()
	mb.server.Close()
}

// waitForMessages collects n messages from the backend with a timeout.
func (mb *mockBackend) waitForMessages(t *testing.T, n int, timeout time.Duration) [][]byte {
	t.Helper()
	var result [][]byte
	deadline := time.After(timeout)
	for len(result) < n {
		select {
		case msg := <-mb.msgCh:
			result = append(result, msg)
		case <-deadline:
			t.Fatalf("timed out waiting for %d messages, got %d", n, len(result))
		}
	}
	return result
}

func TestServer_SendMessage_Success(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handler := func(_ context.Context, taskID string, _ json.RawMessage) (*TaskResult, error) {
		return &TaskResult{
			Artifacts: []Artifact{{Name: "output", Data: "hello from " + taskID}},
		}, nil
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

	// Patch the backend URL — server.Start builds /ws/a2a path from BackendURL.
	// Our mock already handles /ws/a2a, and wsURL returns the raw ws:// URL.
	// Override config so Start constructs the correct URL.
	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))

	// Wait for the agent card registration message.
	regMsgs := mb.waitForMessages(t, 1, 3*time.Second)
	var regReq JSONRPCRequest
	require.NoError(t, json.Unmarshal(regMsgs[0], &regReq))
	assert.Equal(t, MethodRegisterCard, regReq.Method)

	// Send a task to the server.
	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:  "task-001",
			Payload: json.RawMessage(`{"prompt":"test"}`),
			SecurityPolicy: SecurityPolicy{
				AllowNetwork: true,
				AllowFS:      false,
				TimeoutSec:   60,
			},
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	// Expect: working status + completed status + result response = 3 messages.
	msgs := mb.waitForMessages(t, 3, 5*time.Second)

	// Verify working status notification.
	var workingNotif JSONRPCNotification
	require.NoError(t, json.Unmarshal(msgs[0], &workingNotif))
	assert.Equal(t, MethodStatusUpdate, workingNotif.Method)

	// Verify the final response contains completed result.
	var finalResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[2], &finalResp))
	assert.Nil(t, finalResp.Error)

	resultBytes, _ := json.Marshal(finalResp.Result)
	var result TaskResult
	require.NoError(t, json.Unmarshal(resultBytes, &result))
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, "hello from task-001", result.Artifacts[0].Data)
}

func TestServer_SendMessage_HandlerError(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	handler := func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
		return &TaskResult{
			SessionID:     "session-err",
			TraceID:       "trace-err",
			CorrelationID: "corr-err",
		}, fmt.Errorf("handler exploded")
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"fail"},
		Handler:    handler,
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

	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "task-err",
			Payload:        json.RawMessage(`{}`),
			SecurityPolicy: SecurityPolicy{TimeoutSec: 30},
		}),
	}
	data, _ := json.Marshal(taskReq)
	require.NoError(t, mb.sendMessage(data))

	// working status + failed status + result = 3.
	msgs := mb.waitForMessages(t, 3, 5*time.Second)

	var finalResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[2], &finalResp))
	resultBytes, _ := json.Marshal(finalResp.Result)
	var result TaskResult
	require.NoError(t, json.Unmarshal(resultBytes, &result))
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Error, "handler exploded")
	assert.Equal(t, "session-err", result.SessionID)
	assert.Equal(t, "trace-err", result.TraceID)
	assert.Equal(t, "corr-err", result.CorrelationID)
}
