package omp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func ompNativeRequestHasAuth(header http.Header) bool {
	for name, values := range header {
		lower := strings.ToLower(name)
		if lower == "authorization" || strings.Contains(lower, "api-key") {
			for _, value := range values {
				if strings.TrimSpace(value) != "" {
					return true
				}
			}
		}
	}
	return false
}

func ompNativeLastToolContent(messages []json.RawMessage) string {
	for index := len(messages) - 1; index >= 0; index-- {
		var message struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		}
		if json.Unmarshal(messages[index], &message) == nil && message.Role == "tool" {
			encoded, _ := json.Marshal(message.Content)
			return string(encoded)
		}
	}
	return ""
}

func ompNativeRequireTokens(value string, tokens ...string) error {
	for _, token := range tokens {
		if !strings.Contains(value, token) {
			return fmt.Errorf("missing %q", token)
		}
	}
	return nil
}

func writeOMPNativeToolSSE(w http.ResponseWriter, sequence int, name string, args any) {
	arguments, _ := json.Marshal(args)
	chunk := map[string]any{
		"id": "chatcmpl-native", "object": "chat.completion.chunk", "created": sequence, "model": ompLiveModel,
		"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"index": 0, "id": fmt.Sprintf("native-call-%d", sequence), "type": "function",
				"function": map[string]any{"name": name, "arguments": string(arguments)},
			}}},
		}},
	}
	terminal := map[string]any{
		"id": "chatcmpl-native", "object": "chat.completion.chunk", "created": sequence, "model": ompLiveModel,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12},
	}
	writeOMPSSE(w, chunk, terminal)
}

func writeOMPNativeFinalSSE(w http.ResponseWriter, sequence int, content string) {
	chunk := map[string]any{
		"id": "chatcmpl-native", "object": "chat.completion.chunk", "created": sequence, "model": ompLiveModel,
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{"role": "assistant", "content": content},
		}},
	}
	terminal := map[string]any{
		"id": "chatcmpl-native", "object": "chat.completion.chunk", "created": sequence, "model": ompLiveModel,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12},
	}
	writeOMPSSE(w, chunk, terminal)
}
