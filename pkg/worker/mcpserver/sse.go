// Package mcpserver provides SSE transport support for the MCP server.
package mcpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// sseDelivery is a message delivery unit sent to a client.
type sseDelivery struct {
	data []byte
	done chan struct{} // closed when the client goroutine has written the data
}

// sseClient represents a single connected SSE client.
type sseClient struct {
	ch chan *sseDelivery
}

// sseHub manages connected SSE clients and broadcast operations.
type sseHub struct {
	mu      sync.Mutex
	clients []*sseClient
}

func (h *sseHub) add(c *sseClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients = append(h.clients, c)
}

func (h *sseHub) remove(c *sseClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, cl := range h.clients {
		if cl == c {
			h.clients = append(h.clients[:i], h.clients[i+1:]...)
			return
		}
	}
}

// broadcast sends data to all connected clients and waits for each client to
// finish writing before returning. This ensures that callers of BroadcastSSE
// observe a happens-before relationship with subsequent reads of the response body.
func (h *sseHub) broadcast(data []byte) {
	h.mu.Lock()
	// Snapshot clients under the lock, then release it before waiting.
	clients := make([]*sseClient, len(h.clients))
	copy(clients, h.clients)
	h.mu.Unlock()

	deliveries := make([]*sseDelivery, 0, len(clients))
	for _, cl := range clients {
		d := &sseDelivery{data: data, done: make(chan struct{})}
		select {
		case cl.ch <- d:
			deliveries = append(deliveries, d)
		default:
			// @AX:WARN[AUTO]: silent drop on full channel — slow SSE clients lose events without any error signal; consider adding a drop counter metric
			// Drop if channel is full to avoid blocking.
		}
	}
	// Wait for each delivery to be written before returning.
	for _, d := range deliveries {
		<-d.done
	}
}

// getHub returns the sseHub associated with s, creating it on first call.
// @AX:WARN[AUTO]: global state mutation — sseHubs is a package-level map; entries are never removed, causing a memory leak when MCPServer instances are discarded
var (
	sseStateMu sync.Mutex
	sseHubs    = make(map[*MCPServer]*sseHub)
)

func getHub(s *MCPServer) *sseHub {
	sseStateMu.Lock()
	defer sseStateMu.Unlock()
	h, ok := sseHubs[s]
	if !ok {
		h = &sseHub{}
		sseHubs[s] = h
	}
	return h
}

// SSEHandler returns an http.Handler for the /mcp/sse endpoint.
// The handler keeps the connection alive and streams SSE events to the client.
func (s *MCPServer) SSEHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers before writing any body.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Flush headers immediately if the ResponseWriter supports it.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		client := &sseClient{ch: make(chan *sseDelivery, 16)}
		hub := getHub(s)
		hub.add(client)
		defer hub.remove(client)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-client.ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", d.data)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				close(d.done)
			}
		}
	})
}

// BroadcastSSE encodes v as JSON and broadcasts it as an SSE event to all
// connected clients. It blocks until all clients have written the event.
// It is safe for concurrent use.
// @AX:ANCHOR[AUTO]: public API contract — BroadcastSSE is the sole SSE emission path; signature changes break all callers
func (s *MCPServer) BroadcastSSE(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("mcpserver: BroadcastSSE marshal: %w", err)
	}
	getHub(s).broadcast(data)
	return nil
}
