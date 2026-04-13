package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupToolTestServer(t *testing.T) (*MCPServer, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header on all requests.
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		switch {
		case r.URL.Path == "/api/v1/tasks" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"id": "task-1", "status": "created"})

		case r.URL.Path == "/api/v1/workspaces/ws-test/knowledge/search" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    []map[string]string{{"title": "result1"}},
			})

		case r.URL.Path == "/api/v1/executions/exec-1" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"id": "exec-1", "status": "running"})

		case r.URL.Path == "/api/v1/agents" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]string{{"name": "agent1"}})

		case r.URL.Path == "/api/v1/executions/exec-1/approve" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "approved"})

		case r.URL.Path == "/api/v1/workspaces/ws-test" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"id": "ws-test"})

		case r.URL.Path == "/api/v1/workspaces/ws-test" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"id": "ws-test", "updated": "true"})

		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))

	s := &MCPServer{
		backendURL:  srv.URL,
		authToken:   "test-token",
		workspaceID: "ws-test",
		tools:       make(map[string]ToolHandler),
	}
	s.registerTools()
	return s, srv
}

func TestHandleExecuteTask(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	params := json.RawMessage(`{"description":"do something"}`)
	result, err := s.handleExecuteTask(context.Background(), params)
	require.NoError(t, err)

	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "task-1", m["id"])
}

func TestHandleSearchKnowledge(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	params := json.RawMessage(`{"query":"deploy","limit":5}`)
	result, err := s.handleSearchKnowledge(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHandleSearchKnowledge_InvalidParams(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	_, err := s.handleSearchKnowledge(context.Background(), json.RawMessage(`{bad`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid params")
}

func TestHandleGetExecutionStatus(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	params := json.RawMessage(`{"id":"exec-1"}`)
	result, err := s.handleGetExecutionStatus(context.Background(), params)
	require.NoError(t, err)

	m := result.(map[string]any)
	assert.Equal(t, "running", m["status"])
}

func TestHandleGetExecutionStatus_MissingID(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	_, err := s.handleGetExecutionStatus(context.Background(), json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

func TestHandleListAgents(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	result, err := s.handleListAgents(context.Background(), nil)
	require.NoError(t, err)

	agents, ok := result.([]any)
	require.True(t, ok)
	assert.Len(t, agents, 1)
}

func TestHandleApproveExecution(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	params := json.RawMessage(`{"id":"exec-1"}`)
	result, err := s.handleApproveExecution(context.Background(), params)
	require.NoError(t, err)

	m := result.(map[string]any)
	assert.Equal(t, "approved", m["status"])
}

func TestHandleManageWorkspace_Get(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	params := json.RawMessage(`{"action":"get"}`)
	result, err := s.handleManageWorkspace(context.Background(), params)
	require.NoError(t, err)

	m := result.(map[string]any)
	assert.Equal(t, "ws-test", m["id"])
}

func TestHandleManageWorkspace_Update(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	params := json.RawMessage(`{"action":"update","data":{"name":"new"}}`)
	result, err := s.handleManageWorkspace(context.Background(), params)
	require.NoError(t, err)

	m := result.(map[string]any)
	assert.Equal(t, "true", m["updated"])
}

func TestHandleManageWorkspace_InvalidParams(t *testing.T) {
	t.Parallel()
	s, srv := setupToolTestServer(t)
	defer srv.Close()

	_, err := s.handleManageWorkspace(context.Background(), json.RawMessage(`not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid params")
}

func TestExecuteRequest_BackendError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service down"))
	}))
	defer srv.Close()

	s := &MCPServer{backendURL: srv.URL, authToken: "tok"}
	_, err := s.doGet(context.Background(), "/api/v1/health")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backend error 503")
}

func TestExtractID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr string
	}{
		{"valid id", `{"id":"abc"}`, "abc", ""},
		{"empty id", `{"id":""}`, "", "id is required"},
		{"missing id", `{}`, "", "id is required"},
		{"invalid json", `{bad`, "", "invalid params"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, err := extractID(json.RawMessage(tt.input))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}
