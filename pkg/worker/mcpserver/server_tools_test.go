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

	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, first["name"])
	assert.NotEmpty(t, first["title"])
	assert.NotEmpty(t, first["description"])
	_, hasSchema := first["inputSchema"]
	assert.True(t, hasSchema, "tool descriptors should expose inputSchema")
	_, hasAnnotations := first["annotations"]
	assert.True(t, hasAnnotations, "tool descriptors should expose annotations")
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

func TestMCPServer_ToolsCallSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "agent1"}})
	}))
	defer srv.Close()

	s, buf := newTestServer(srv.URL)
	params, _ := json.Marshal(map[string]string{"name": "list_agents"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(10), Method: "tools/call", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error, "successful tool call should not error")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	_, hasIsError := result["isError"]
	assert.False(t, hasIsError, "successful tool result should not set isError")
	content, ok := result["content"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, content)
}

func TestMCPServer_ToolsCallHandlerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
	}))
	defer srv.Close()

	s, buf := newTestServer(srv.URL)
	params, _ := json.Marshal(map[string]string{"name": "list_agents"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(11), Method: "tools/call", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	assert.True(t, result["isError"].(bool))
	content, ok := result["content"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, content)
}
