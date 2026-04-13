package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates an MCPServer that writes to a buffer instead of stdout.
func newTestServer(backendURL string) (*MCPServer, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	s := &MCPServer{
		backendURL:  backendURL,
		authToken:   "test-token",
		workspaceID: "ws-test",
		tools:       make(map[string]ToolHandler),
		writer:      buf,
	}
	s.resources = NewResourceRegistry(backendURL, "test-token")
	s.registerTools()
	return s, buf
}

func parseResponse(t *testing.T, buf *bytes.Buffer) jsonRPCResponse {
	t.Helper()
	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	return resp
}

func TestMCPServer_Initialize(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "initialize"}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, float64(1), resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2024-11-05", result["protocolVersion"])

	serverInfo := result["serverInfo"].(map[string]any)
	assert.Equal(t, "autopus-adk", serverInfo["name"])
}

func TestMCPServer_ToolsList(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(2), Method: "tools/list"}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)

	tools, ok := result["tools"].([]any)
	require.True(t, ok)
	assert.Len(t, tools, 6, "should have 6 registered tools")
}

func TestMCPServer_UnknownMethod(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(3), Method: "bogus/method"}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "method not found")
}

func TestMCPServer_InitializedNotification_IsIgnored(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}
	s.dispatch(t.Context(), req)

	assert.Empty(t, buf.String(), "notifications must not produce a JSON-RPC response")
}

func TestMCPServer_UnknownNotification_IsIgnored(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", Method: "notifications/progress"}
	s.dispatch(t.Context(), req)

	assert.Empty(t, buf.String(), "unknown notifications must be ignored")
}

func TestMCPServer_Ping(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(8), Method: "ping"}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	assert.Equal(t, float64(8), resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, result)
}

func TestMCPServer_ToolsCallUnknownTool(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	params, _ := json.Marshal(map[string]string{"name": "nonexistent"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(4), Method: "tools/call", Params: params}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestMCPServer_ToolsCallInvalidParams(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(5), Method: "tools/call", Params: json.RawMessage(`invalid`)}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestMCPServer_ResourcesList(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(6), Method: "resources/list"}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	resources, ok := result["resources"].([]any)
	require.True(t, ok)
	assert.Len(t, resources, 4, "should have 4 default resources")
}

func TestMCPServer_ResourcesReadInvalidParams(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(7), Method: "resources/read", Params: json.RawMessage(`{bad`)}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestMCPServer_SendError(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	s.sendError("req-1", -32600, "invalid request")

	resp := parseResponse(t, buf)
	assert.Equal(t, "req-1", resp.ID)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32600, resp.Error.Code)
	assert.Equal(t, "invalid request", resp.Error.Message)
}

func TestMCPServer_ToolsCallSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]string{{"name": "agent1"}})
	}))
	defer srv.Close()

	s, buf := newTestServer(srv.URL)
	params, _ := json.Marshal(map[string]string{"name": "list_agents"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(10), Method: "tools/call", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error, "successful tool call should not error")
	assert.NotNil(t, resp.Result)
}

func TestMCPServer_ToolsCallHandlerError(t *testing.T) {
	t.Parallel()

	// Use a server that returns 500 to trigger handler error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	s, buf := newTestServer(srv.URL)
	params, _ := json.Marshal(map[string]string{"name": "list_agents"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(11), Method: "tools/call", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32000, resp.Error.Code)
}

func TestMCPServer_ResourcesReadSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}))
	defer srv.Close()

	s, buf := newTestServer(srv.URL)
	params, _ := json.Marshal(map[string]string{"uri": "autopus://status"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(12), Method: "resources/read", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestMCPServer_ResourcesReadUnknown(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	params, _ := json.Marshal(map[string]string{"uri": "autopus://nonexistent"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(13), Method: "resources/read", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32000, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "unknown resource")
}
