package cli

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequentialStructuredReviewBackend struct {
	mu       sync.Mutex
	requests []orchestra.ProviderRequest
	outputs  []orchestra.ProviderResponse
}

func (b *sequentialStructuredReviewBackend) Execute(_ context.Context, req orchestra.ProviderRequest) (*orchestra.ProviderResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.requests = append(b.requests, req)
	idx := len(b.requests) - 1
	resp := b.outputs[len(b.outputs)-1]
	if idx < len(b.outputs) {
		resp = b.outputs[idx]
	}
	resp.Provider = req.Provider
	if resp.Duration == 0 {
		resp.Duration = 5 * time.Millisecond
	}
	resp.ExecutedBackend = b.Name()
	return &resp, nil
}

func (b *sequentialStructuredReviewBackend) Name() string {
	return "subprocess"
}

func TestRunStructuredSpecReviewOrchestra_RepromptsMalformedGeminiReviewerOutput(t *testing.T) {
	backend := &sequentialStructuredReviewBackend{
		outputs: []orchestra.ProviderResponse{
			{Output: "SPEC-AGENT-ORG-WIRING-001 looks mostly complete, but the document needs a few edits."},
			{Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers: []orchestra.ProviderConfig{{
			Name:          "gemini",
			Binary:        "agy",
			Args:          []string{"--print", ""},
			PromptViaArgs: true,
			OutputFormat:  "text",
		}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	assert.Empty(t, result.FailedProviders)
	assert.Equal(t, `{"verdict":"PASS","summary":"ok","findings":[]}`, result.Responses[0].Output)

	backend.mu.Lock()
	defer backend.mu.Unlock()
	require.Len(t, backend.requests, 2)
	assert.Contains(t, backend.requests[1].Prompt, "Your previous reviewer response was not valid JSON")
	assert.Contains(t, backend.requests[1].Prompt, "Return exactly one JSON object")
	assert.Contains(t, backend.requests[1].Prompt, "Required JSON schema")
}
