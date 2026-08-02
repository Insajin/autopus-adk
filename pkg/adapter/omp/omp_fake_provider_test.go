package omp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const (
	ompLiveModel   = "s7-probe"
	ompReceiptName = "fixture-receipt.json"
	ompReceiptJSON = `{"command":"auto-plan","context_profile":"plan","status":"completed"}`
)

type ompFakeStep struct {
	gateName  string
	needles   []string
	exactBody string
	toolName  string
	toolArgs  map[string]string
}

type ompFakeProvider struct {
	server *httptest.Server

	mu                         sync.Mutex
	steps                      []ompFakeStep
	requestCount               int
	stage                      int
	failure                    string
	loopbackAuthHeaders        int
	unexpectedLoopbackEndpoint int
}

func newOMPFakeProvider(t *testing.T, commandNeedles []string, commandBody, autoBody, planBody string) *ompFakeProvider {
	t.Helper()
	provider := &ompFakeProvider{steps: []ompFakeStep{
		{
			gateName:  "expanded-command",
			needles:   commandNeedles,
			exactBody: commandBody,
			toolName:  "read",
			toolArgs:  map[string]string{"path": "skill://auto"},
		},
		{
			gateName: "auto-skill-body",
			needles: []string{
				"## Router Contract",
				"Do not fuzzy-correct an unknown subcommand",
				"`auto-plan`",
			},
			exactBody: autoBody,
			toolName:  "read",
			toolArgs:  map[string]string{"path": "skill://auto-plan"},
		},
		{
			gateName: "auto-plan-skill-body",
			needles: []string{
				"## Context Profile: plan",
				"core,architecture,relevant_spec",
				"test,canary",
			},
			exactBody: planBody,
			toolName:  "write",
			toolArgs:  map[string]string{"path": ompReceiptName, "content": ompReceiptJSON},
		},
		{
			gateName: "bounded-write-result",
			needles:  []string{ompReceiptName, "tool"},
		},
	}}
	provider.server = httptest.NewServer(http.HandlerFunc(provider.handle))
	t.Cleanup(provider.server.Close)
	return provider
}

func (p *ompFakeProvider) handle(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requestCount++

	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		p.unexpectedLoopbackEndpoint++
		p.fail(w, "unexpected_endpoint")
		return
	}
	if r.Header.Get("Authorization") != "" {
		p.loopbackAuthHeaders++
		p.fail(w, "unexpected_auth_header")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		p.fail(w, "request_oversized")
		return
	}
	var request struct {
		Model    string            `json:"model"`
		Stream   bool              `json:"stream"`
		Tools    []json.RawMessage `json:"tools"`
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &request) != nil || request.Model != ompLiveModel || !request.Stream {
		p.fail(w, "invalid_request_shape")
		return
	}
	if p.stage >= len(p.steps) {
		p.fail(w, "request_budget_exceeded")
		return
	}
	step := p.steps[p.stage]
	for _, needle := range step.needles {
		if !strings.Contains(string(body), needle) {
			p.fail(w, "content_gate_failed:"+step.gateName)
			return
		}
	}
	if step.exactBody != "" && !requestContainsExactBody(request.Messages, step.exactBody) {
		p.fail(w, "body_hash_gate_failed:"+step.gateName)
		return
	}
	if step.toolName != "" && !requestDeclaresTool(request.Tools, step.toolName) {
		p.fail(w, "tool_not_declared:"+step.toolName)
		return
	}
	p.stage++
	if step.toolName == "" {
		writeOMPFinalSSE(w, p.stage)
		return
	}
	writeOMPToolSSE(w, p.stage, step.toolName, step.toolArgs)
}

func requestContainsExactBody(messages []json.RawMessage, expected string) bool {
	for _, raw := range messages {
		var message any
		if json.Unmarshal(raw, &message) == nil && jsonValueContainsString(message, expected) {
			return true
		}
	}
	return false
}

func jsonValueContainsString(value any, expected string) bool {
	switch typed := value.(type) {
	case string:
		return strings.Contains(typed, expected)
	case []any:
		for _, item := range typed {
			if jsonValueContainsString(item, expected) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if jsonValueContainsString(item, expected) {
				return true
			}
		}
	}
	return false
}

func (p *ompFakeProvider) fail(w http.ResponseWriter, reason string) {
	if p.failure == "" {
		p.failure = reason
	}
	http.Error(w, "fixture rejected request", http.StatusBadRequest)
}

func (p *ompFakeProvider) receipt() (
	requests, stages, loopbackAuthHeaders, unexpectedLoopbackEndpoints int,
	failure string,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requestCount, p.stage, p.loopbackAuthHeaders, p.unexpectedLoopbackEndpoint, p.failure
}

func requestDeclaresTool(tools []json.RawMessage, name string) bool {
	for _, raw := range tools {
		var tool struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if json.Unmarshal(raw, &tool) == nil && tool.Function.Name == name {
			return true
		}
	}
	return false
}

func writeOMPToolSSE(w http.ResponseWriter, sequence int, name string, args map[string]string) {
	arguments, _ := json.Marshal(args)
	chunk := map[string]any{
		"id": "chatcmpl-fixture", "object": "chat.completion.chunk", "created": 1, "model": ompLiveModel,
		"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{
				"role": "assistant",
				"tool_calls": []any{map[string]any{
					"index": 0, "id": fmt.Sprintf("call-%d", sequence), "type": "function",
					"function": map[string]any{"name": name, "arguments": string(arguments)},
				}},
			},
		}},
	}
	terminal := map[string]any{
		"id": "chatcmpl-fixture", "object": "chat.completion.chunk", "created": 1, "model": ompLiveModel,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12},
	}
	writeOMPSSE(w, chunk, terminal)
}

func writeOMPFinalSSE(w http.ResponseWriter, sequence int) {
	chunk := map[string]any{
		"id": "chatcmpl-fixture", "object": "chat.completion.chunk", "created": sequence, "model": ompLiveModel,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "fixture complete"}}},
	}
	terminal := map[string]any{
		"id": "chatcmpl-fixture", "object": "chat.completion.chunk", "created": sequence, "model": ompLiveModel,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12},
	}
	writeOMPSSE(w, chunk, terminal)
}

func writeOMPSSE(w http.ResponseWriter, chunks ...map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, chunk := range chunks {
		encoded, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}
