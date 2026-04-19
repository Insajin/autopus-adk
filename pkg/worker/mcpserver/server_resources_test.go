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

	first, ok := resources[0].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, first["title"])
}

func TestMCPServer_ResourceTemplatesList(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(6), Method: "resources/templates/list"}
	s.dispatch(t.Context(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	templates, ok := result["resourceTemplates"].([]any)
	require.True(t, ok)
	require.Len(t, templates, 1)

	first, ok := templates[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "autopus://executions/{id}", first["uriTemplate"])
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

func TestMCPServer_ResourcesReadSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}))
	defer srv.Close()

	s, buf := newTestServer(srv.URL)
	params, _ := json.Marshal(map[string]string{"uri": "autopus://status"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(12), Method: "resources/read", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	contents, ok := result["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 1)
	content, ok := contents[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "autopus://status", content["uri"])
	assert.Equal(t, "application/json", content["mimeType"])
	assert.NotEmpty(t, content["text"])
}

func TestMCPServer_ResourcesReadUnknown(t *testing.T) {
	t.Parallel()

	s, buf := newTestServer("http://localhost")
	params, _ := json.Marshal(map[string]string{"uri": "autopus://nonexistent"})
	req := &jsonRPCRequest{JSONRPC: "2.0", ID: float64(13), Method: "resources/read", Params: params}
	s.dispatch(context.Background(), req)

	resp := parseResponse(t, buf)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32002, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "unknown resource")
}
