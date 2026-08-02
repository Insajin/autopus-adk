package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ompReadinessBehaviorTimeout          = 4 * time.Second
	ompReadinessCompletionPath           = "/v1/chat/completions"
	ompReadinessProviderID               = "autopus-readiness"
	ompReadinessModel                    = "readiness-probe"
	ompReadinessReceiptName              = "readiness-receipt.json"
	ompReadinessReceiptContent           = "readiness-ok\n"
	ompReadinessRequestBudget            = 2
	ompReadinessRequestMaxBytes          = 2 << 20
	ompReadinessProviderResponseMaxBytes = 8 << 10
)

type ompReadinessBehaviorProvider struct {
	listener net.Listener
	server   *http.Server

	mu          sync.Mutex
	requests    int
	stages      int
	authHeaders int
	failure     string
}

func runOMPReadinessBehavioralProbe(
	parent context.Context,
	opts OMPReadinessOptions,
	runner commandOMPProbeRunner,
	overlay string,
) ompProbeResult {
	if !supportsOMPReadinessBehaviorProcessGroup() {
		return ompProbeResult{reason: "output_invalid"}
	}
	base := filepath.Dir(overlay)
	scratch := filepath.Join(base, "scratch")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		return ompProbeResult{reason: "output_invalid"}
	}
	receipt := filepath.Join(scratch, ompReadinessReceiptName)
	provider, err := startOMPReadinessBehaviorProvider(scratch)
	if err != nil {
		return ompProbeResult{reason: "output_invalid"}
	}
	defer provider.Close()
	if err := writeOMPReadinessModelConfig(filepath.Join(base, "pi-agent"), provider.URL()); err != nil {
		return ompProbeResult{reason: "output_invalid"}
	}

	stdin := ompReadinessRPCInput()
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-002: the deterministic readiness RPC request is capped at 1 KiB before launch.
	if len(stdin) > 1024 {
		return ompProbeResult{reason: "output_invalid"}
	}
	ctx, cancel := context.WithTimeout(normalizeOMPReadinessContext(parent), ompReadinessBehaviorTimeout)
	defer cancel()
	output, runErr := runner.runRPC(ctx, opts.Executable, scratch, stdin,
		"--config", overlay,
		"--mode", "rpc",
		"--no-session",
		"--cwd", scratch,
		"--model", ompReadinessProviderID+"/"+ompReadinessModel,
		"--tools", "write",
		"--auto-approve",
		"--no-extensions",
		"--no-rules",
		"--no-lsp",
		"--no-pty",
	)
	if runErr != nil {
		return ompProbeResult{output: output, reason: classifyOMPProbeError(ctx, runErr)}
	}
	if len(output) > opts.MaxOutput || !provider.healthy() || !secureOMPReadinessReceipt(receipt) {
		return ompProbeResult{reason: "output_invalid"}
	}
	return ompProbeResult{output: output}
}

func normalizeOMPReadinessContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func ompReadinessRPCInput() []byte {
	frames := []map[string]any{
		{"type": "set_auto_retry", "enabled": false},
		{"type": "set_auto_compaction", "enabled": false},
		{"type": "prompt", "message": "Complete the bounded readiness write."},
	}
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, frame := range frames {
		_ = encoder.Encode(frame)
	}
	return input.Bytes()
}

func startOMPReadinessBehaviorProvider(scratch string) (*ompReadinessBehaviorProvider, error) {
	info, err := os.Stat(scratch)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return nil, errors.New("invalid readiness scratch")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	provider := &ompReadinessBehaviorProvider{listener: listener}
	provider.server = &http.Server{
		Handler:           http.HandlerFunc(provider.handle),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
	}
	// @AX:WARN [AUTO]: the loopback provider goroutine is not governed by a context.Context.
	// @AX:REASON [AUTO]: its lifecycle relies on the caller's deferred provider.Close to stop Serve and release the listener.
	go func() { _ = provider.server.Serve(listener) }()
	return provider, nil
}

func (p *ompReadinessBehaviorProvider) URL() string {
	return "http://" + p.listener.Addr().String()
}

func (p *ompReadinessBehaviorProvider) Close() {
	_ = p.server.Close()
}

func (p *ompReadinessBehaviorProvider) FailureReason() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failure
}

func (p *ompReadinessBehaviorProvider) healthy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failure == "" && p.requests == ompReadinessRequestBudget &&
		p.stages == ompReadinessRequestBudget && p.authHeaders == 0
}

func (p *ompReadinessBehaviorProvider) handle(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests++
	if p.requests > ompReadinessRequestBudget {
		p.reject(w, "request_budget_exceeded")
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != ompReadinessCompletionPath {
		p.reject(w, "unexpected_endpoint")
		return
	}
	if _, present := r.Header["Authorization"]; present {
		p.authHeaders++
		p.reject(w, "authorization_present")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, ompReadinessRequestMaxBytes))
	if err != nil {
		p.reject(w, "request_oversized")
		return
	}
	if !validOMPReadinessProviderRequest(body, p.stages) {
		p.reject(w, "invalid_request")
		return
	}
	response, err := ompReadinessProviderSSE(p.stages)
	if err != nil {
		p.reject(w, "response_invalid")
		return
	}
	p.stages++
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write(response)
}

func (p *ompReadinessBehaviorProvider) reject(w http.ResponseWriter, reason string) {
	if p.failure == "" {
		p.failure = reason
	}
	http.Error(w, "readiness provider rejected request", http.StatusBadRequest)
}

func validOMPReadinessProviderRequest(body []byte, stage int) bool {
	var request struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
		Tools  []struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if json.Unmarshal(body, &request) != nil || request.Model != ompReadinessModel ||
		!request.Stream || len(request.Tools) != 1 || request.Tools[0].Type != "function" ||
		request.Tools[0].Function.Name != "write" {
		return false
	}
	return stage == 0 || bytes.Contains(body, []byte(ompReadinessReceiptName))
}

func ompReadinessProviderSSE(stage int) ([]byte, error) {
	var chunks []map[string]any
	if stage == 0 {
		arguments, _ := json.Marshal(map[string]string{
			"path": ompReadinessReceiptName, "content": ompReadinessReceiptContent,
		})
		chunks = []map[string]any{
			{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{
				"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "readiness-1",
					"type": "function", "function": map[string]any{"name": "write", "arguments": string(arguments)}}},
			}}}},
			{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}}},
		}
	} else {
		chunks = []map[string]any{
			{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "ready"}}}},
			{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}},
		}
	}
	var response strings.Builder
	for index, chunk := range chunks {
		chunk["id"] = "chatcmpl-readiness"
		chunk["object"] = "chat.completion.chunk"
		chunk["created"] = index + 1
		chunk["model"] = ompReadinessModel
		encoded, err := json.Marshal(chunk)
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprintf(&response, "data: %s\n\n", encoded)
	}
	response.WriteString("data: [DONE]\n\n")
	if response.Len() > ompReadinessProviderResponseMaxBytes {
		return nil, errors.New("readiness response oversized")
	}
	return []byte(response.String()), nil
}

func writeOMPReadinessModelConfig(profile, serverURL string) error {
	content := fmt.Sprintf(`providers:
  %s:
    baseUrl: %s/v1
    auth: none
    api: openai-completions
    models:
      - id: %s
        name: Readiness Probe
        reasoning: false
        input: [text]
        contextWindow: 4096
        maxTokens: 256
`, ompReadinessProviderID, serverURL, ompReadinessModel)
	return os.WriteFile(filepath.Join(profile, "models.yml"), []byte(content), 0o600)
}

func secureOMPReadinessReceipt(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return false
	}
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return false
	}
	content, err := os.ReadFile(path)
	return err == nil && string(content) == ompReadinessReceiptContent
}
