// Package mcpserver_test tests the SSE transport endpoint for the MCP server.
package mcpserver_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSEHandler_Connection verifies that the SSE endpoint establishes a connection.
// Uses httptest.Server + real HTTP client to avoid concurrent access to ResponseRecorder.
func TestSSEHandler_Connection(t *testing.T) {
	t.Parallel()

	// Given: an MCP server with an SSE handler registered
	srv := mcpserver.New("", "", "")
	ts := httptest.NewServer(srv.SSEHandler())
	defer ts.Close()

	// When: making a GET request to the SSE endpoint
	client := ts.Client()
	resp, err := client.Get(ts.URL + "/mcp/sse")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Then: the response uses SSE content type and status 200
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

// TestSSEHandler_ContentType verifies Content-Type is text/event-stream.
func TestSSEHandler_ContentType(t *testing.T) {
	t.Parallel()

	// Given: an MCP server with an SSE handler
	srv := mcpserver.New("", "", "")
	ts := httptest.NewServer(srv.SSEHandler())
	defer ts.Close()

	// When: a client connects
	client := ts.Client()
	resp, err := client.Get(ts.URL + "/mcp/sse")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Then: Content-Type must be text/event-stream
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"),
		"SSE endpoint must respond with Content-Type: text/event-stream")
}

// TestSSEHandler_SendEvent verifies that a JSON-RPC message is sent as an SSE event.
func TestSSEHandler_SendEvent(t *testing.T) {
	t.Parallel()

	// Given: an MCP server with an SSE handler and a connected real HTTP client
	srv := mcpserver.New("", "", "")
	ts := httptest.NewServer(srv.SSEHandler())
	defer ts.Close()

	client := ts.Client()
	resp, err := client.Get(ts.URL + "/mcp/sse")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Allow the server to register the SSE client before broadcasting.
	time.Sleep(30 * time.Millisecond)

	// When: a JSON-RPC message is broadcast through the SSE transport
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  map[string]any{"status": "ok"},
	}
	err = srv.BroadcastSSE(msg)
	require.NoError(t, err)

	// Then: read one SSE event from the response body stream
	reader := bufio.NewReader(resp.Body)
	var dataLine string
	deadline := time.After(2 * time.Second)
	lineCh := make(chan string, 1)
	go func() {
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil && readErr != io.EOF {
				return
			}
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				lineCh <- strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				return
			}
		}
	}()

	select {
	case dataLine = <-lineCh:
	case <-deadline:
		t.Fatal("timed out waiting for SSE data event")
	}

	require.NotEmpty(t, dataLine, "SSE data line must not be empty")

	var decoded map[string]any
	err = json.Unmarshal([]byte(dataLine), &decoded)
	require.NoError(t, err, "SSE data must be valid JSON")
	assert.Equal(t, "2.0", decoded["jsonrpc"])
}
