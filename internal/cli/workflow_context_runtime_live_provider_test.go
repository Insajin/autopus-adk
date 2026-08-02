package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

const workflowContextLiveModel = "context-canary"

type workflowContextLiveProvider struct {
	server *httptest.Server

	mu                  sync.Mutex
	requests            int
	authHeaders         int
	unexpectedEndpoints int
	failure             string
}

func newWorkflowContextLiveProvider(t *testing.T) *workflowContextLiveProvider {
	t.Helper()
	provider := &workflowContextLiveProvider{}
	provider.server = httptest.NewServer(http.HandlerFunc(provider.handle))
	t.Cleanup(provider.server.Close)
	return provider
}

func (p *workflowContextLiveProvider) URL() string { return p.server.URL }

func (p *workflowContextLiveProvider) handle(w http.ResponseWriter, request *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests++
	if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
		p.unexpectedEndpoints++
		p.reject(w, "unexpected-endpoint")
		return
	}
	if request.Header.Get("Authorization") != "" {
		p.authHeaders++
		p.reject(w, "unexpected-auth")
		return
	}
	var body struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4<<20))
	if decoder.Decode(&body) != nil || body.Model != workflowContextLiveModel || !body.Stream {
		p.reject(w, "invalid-request-shape")
		return
	}
	promptTokens := 4096
	if p.requests == 2 {
		promptTokens = 7800
	}
	w.Header().Set("Content-Type", "text/event-stream")
	chunk := map[string]any{
		"id": "context-canary", "object": "chat.completion.chunk", "created": 1,
		"model": workflowContextLiveModel,
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{"role": "assistant", "content": "completed"},
		}},
	}
	terminal := map[string]any{
		"id": "context-canary", "object": "chat.completion.chunk", "created": 1,
		"model": workflowContextLiveModel,
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{}, "finish_reason": "stop",
		}},
		"usage": map[string]int{
			"prompt_tokens": promptTokens, "completion_tokens": 8, "total_tokens": promptTokens + 8,
		},
	}
	for _, value := range []map[string]any{chunk, terminal} {
		encoded, _ := json.Marshal(value)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}

func (p *workflowContextLiveProvider) reject(w http.ResponseWriter, reason string) {
	if p.failure == "" {
		p.failure = reason
	}
	http.Error(w, "fixture rejected request", http.StatusBadRequest)
}

func (p *workflowContextLiveProvider) receipt() (requests, auth, endpoints int, failure string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requests, p.authHeaders, p.unexpectedEndpoints, p.failure
}
