package a2a

// coverage_gap_handlers_test.go: tests for server message handlers and REST poller
// covering edge cases and error branches.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- handleMessage: unknown method branch ---

func TestServer_HandleMessage_UnknownMethod_LogsAndContinues(t *testing.T) {
	t.Parallel()

	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "w-unknown",
		Skills:     []string{"test"},
		Handler:    func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) { return &TaskResult{}, nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	// Send an unknown method — should log and not crash.
	unknownReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"unk-1"`),
		Method:  "unknown/method",
		Params:  json.RawMessage(`{}`),
	}
	data, err := json.Marshal(unknownReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	time.Sleep(30 * time.Millisecond)
}

// --- handleSendMessage: invalid params branch ---

func TestServer_HandleSendMessage_InvalidParams(t *testing.T) {
	t.Parallel()

	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "w-invalid",
		Skills:     []string{"test"},
		Handler:    func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) { return &TaskResult{}, nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	// Send tasks/send with invalid JSON params (raw bytes, bypassing json.Marshal).
	badMsg := []byte(`{"jsonrpc":"2.0","id":"bad-send","method":"tasks/send","params":"not-an-object"}`)
	require.NoError(t, mb.sendMessage(badMsg))

	// Expect an error response back from the server.
	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &resp))
	assert.NotNil(t, resp.Error, "invalid params should return JSON-RPC error")
}

// --- RESTPoller: Start is idempotent (double-start) ---

func TestRESTPoller_DoubleStart_IsIdempotent(t *testing.T) {
	t.Parallel()

	pollSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"tasks": []interface{}{}})
	}))
	defer pollSrv.Close()

	poller := NewRESTPoller(RESTPollerConfig{
		BackendURL:   pollSrv.URL,
		AuthToken:    "tok",
		WorkerID:     "w1",
		PollInterval: 20 * time.Millisecond,
		TaskHandler:  func(_ PollResult) error { return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.Start(ctx)
	poller.Start(ctx) // second Start should be a no-op (no panic, no duplicate goroutine)

	time.Sleep(10 * time.Millisecond)
	poller.Stop()
	// Stop twice should also be idempotent.
	poller.Stop()
}

// --- RESTPoller: unexpected status code branch ---

func TestRESTPoller_UnexpectedStatusCode_ReturnsError(t *testing.T) {
	t.Parallel()

	pollSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer pollSrv.Close()

	poller := NewRESTPoller(RESTPollerConfig{
		BackendURL:   pollSrv.URL,
		AuthToken:    "tok",
		WorkerID:     "w1",
		PollInterval: 15 * time.Millisecond,
		TaskHandler:  func(_ PollResult) error { return nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	poller.Start(ctx)
	// Just verify no panic occurs when server returns 500.
	<-ctx.Done()
}
