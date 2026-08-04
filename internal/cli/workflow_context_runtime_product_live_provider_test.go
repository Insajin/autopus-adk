package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newWorkflowContextProductLiveProvider(t *testing.T) *workflowContextLiveProvider {
	t.Helper()
	provider := &workflowContextLiveProvider{}
	provider.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		provider.mu.Lock()
		defer provider.mu.Unlock()
		provider.requests++
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
			provider.unexpectedEndpoints++
			provider.reject(w, "unexpected-endpoint")
			return
		}
		if request.Header.Get("Authorization") != "Bearer "+workflowContextProductLiveCredential {
			provider.reject(w, "invalid-auth")
			return
		}
		provider.authHeaders++
		var body struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(io.LimitReader(request.Body, 4<<20)).Decode(&body) != nil ||
			body.Model != workflowContextLiveModel || !body.Stream {
			provider.reject(w, "invalid-request-shape")
			return
		}
		for index := len(body.Messages) - 1; index >= 0; index-- {
			if body.Messages[index].Role == "user" {
				provider.userMessages = append(provider.userMessages, workflowContextLiveMessageText(body.Messages[index].Content))
				break
			}
		}
		promptTokens := 4096
		if provider.requests == 2 || provider.requests == 4 {
			promptTokens = 110000
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, value := range []map[string]any{
			{"id": "product-live", "object": "chat.completion.chunk", "created": 1, "model": workflowContextLiveModel,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "completed"}}}},
			{"id": "product-live", "object": "chat.completion.chunk", "created": 1, "model": workflowContextLiveModel,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
				"usage":   map[string]int{"prompt_tokens": promptTokens, "completion_tokens": 8, "total_tokens": promptTokens + 8}},
		} {
			encoded, _ := json.Marshal(value)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(provider.server.Close)
	return provider
}
