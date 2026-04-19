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
		return nil, fmt.Errorf("handler exploded")
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
}

func TestServer_ReconnectTransport_ReRegistersAgentCard(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "reconnect-worker",
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
	require.NoError(t, srv.ReconnectTransport(ctx))

	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var regReq JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &regReq))
	assert.Equal(t, MethodRegisterCard, regReq.Method)
}

func TestMergeTaskPayload_InjectsModelAndPipelineMetadata(t *testing.T) {
	payload, err := mergeTaskPayload(
		json.RawMessage(`{"prompt":"hello"}`),
		"gpt-5.4",
		[]string{"planner", "reviewer"},
		map[string]string{"planner": "Plan carefully."},
		map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
		&IterationBudget{Limit: 15, WarnThreshold: 0.7, DangerThreshold: 0.9},
	)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, "gpt-5.4", decoded["model"])
	assert.Equal(t, []interface{}{"planner", "reviewer"}, decoded["pipeline_phases"])
	assert.Equal(t, map[string]any{"planner": "Plan carefully."}, decoded["pipeline_instructions"])
	assert.Equal(t, map[string]any{"planner": "SERVER TEMPLATE\n\n{{input}}"}, decoded["pipeline_prompt_templates"])
	assert.Equal(t, map[string]any{"limit": float64(15), "warn_threshold": 0.7, "danger_threshold": 0.9}, decoded["iteration_budget"])
}

func TestServer_SendMessage_MissingPolicySignatureRejectedWhenSecretConfigured(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "test-secret")

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
	mb.waitForMessages(t, 1, 3*time.Second)

	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "task-no-sig",
			Payload:        json.RawMessage(`{"prompt":"test"}`),
			SecurityPolicy: SecurityPolicy{TimeoutSec: 60},
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var finalResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &finalResp))
	require.NotNil(t, finalResp.Error)
	assert.Contains(t, finalResp.Error.Message, "missing policy signature")
}

func TestServer_SendMessage_MissingControlPlaneSignatureRejectedWhenSecretConfigured(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "test-secret")

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
	mb.waitForMessages(t, 1, 3*time.Second)

	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:                   "task-no-control-plane-sig",
			Payload:                  json.RawMessage(`{"prompt":"test"}`),
			Model:                    "gpt-5.4",
			ControlPlaneCapabilities: []string{CapabilityServerModelV1},
			PolicySignature:          mustSignPolicy(t, "task-no-control-plane-sig", SecurityPolicy{TimeoutSec: 60}, "test-secret"),
			SecurityPolicy:           SecurityPolicy{TimeoutSec: 60},
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var finalResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &finalResp))
	require.NotNil(t, finalResp.Error)
	assert.Contains(t, finalResp.Error.Message, "missing control plane signature")
}

func TestServer_SendMessage_ControlPlaneCapabilitiesFilterMetadata(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "test-secret")

	mb := newMockBackend()
	defer mb.close()

	modelSeen := make(chan string, 1)
	handler := func(_ context.Context, _ string, payload json.RawMessage) (*TaskResult, error) {
		var msg struct {
			Model                   string            `json:"model"`
			PipelinePhases          []string          `json:"pipeline_phases"`
			PipelineInstructions    map[string]string `json:"pipeline_instructions"`
			PipelinePromptTemplates map[string]string `json:"pipeline_prompt_templates"`
			IterationBudget         *IterationBudget  `json:"iteration_budget"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		modelSeen <- msg.Model
		if len(msg.PipelinePhases) != 0 || len(msg.PipelineInstructions) != 0 || len(msg.PipelinePromptTemplates) != 0 || msg.IterationBudget != nil {
			return nil, fmt.Errorf("unexpected unauthorized control plane metadata")
		}
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
	mb.waitForMessages(t, 1, 3*time.Second)

	policy := SecurityPolicy{TimeoutSec: 60}
	controlPlaneCaps := []string{CapabilityServerModelV1}
	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:                   "task-filter-control-plane",
			Payload:                  json.RawMessage(`{"prompt":"test"}`),
			Model:                    "gpt-5.4",
			PipelinePhases:           []string{"planner", "reviewer"},
			PipelineInstructions:     map[string]string{"planner": "Plan carefully."},
			PipelinePromptTemplates:  map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
			IterationBudget:          &IterationBudget{Limit: 12, WarnThreshold: 0.7, DangerThreshold: 0.9},
			ControlPlaneCapabilities: controlPlaneCaps,
			ControlPlaneSignature:    mustSignControlPlane(t, "task-filter-control-plane", "gpt-5.4", []string{"planner", "reviewer"}, map[string]string{"planner": "Plan carefully."}, map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"}, &IterationBudget{Limit: 12, WarnThreshold: 0.7, DangerThreshold: 0.9}, controlPlaneCaps, "test-secret"),
			PolicySignature:          mustSignPolicy(t, "task-filter-control-plane", policy, "test-secret"),
			SecurityPolicy:           policy,
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	select {
	case got := <-modelSeen:
		assert.Equal(t, "gpt-5.4", got)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive filtered control plane metadata")
	}
}

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

	// Send a task first so it's tracked.
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

	// Wait for working status.
	mb.waitForMessages(t, 1, 3*time.Second)

	// Now send cancel.
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

	// Expect: canceled status + cancel result response = 2.
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
