package cli

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type fakeStructuredReviewBackend struct {
	mu       sync.Mutex
	requests []orchestra.ProviderRequest
	outputs  map[string]orchestra.ProviderResponse
}

func (f *fakeStructuredReviewBackend) Execute(_ context.Context, req orchestra.ProviderRequest) (*orchestra.ProviderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	resp := f.outputs[req.Provider]
	resp.Provider = req.Provider
	if resp.Duration == 0 {
		resp.Duration = 5 * time.Millisecond
	}
	return &resp, nil
}

func (f *fakeStructuredReviewBackend) Name() string {
	return "fake-structured-review"
}

func TestRunStructuredSpecReviewOrchestra_InjectsReviewerContract(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"claude": {Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func() orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "claude", Binary: "claude"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	assert.Empty(t, result.FailedProviders)

	backend.mu.Lock()
	defer backend.mu.Unlock()
	require.Len(t, backend.requests, 1)
	assert.Equal(t, "reviewer", backend.requests[0].Role)
	assert.Contains(t, backend.requests[0].Prompt, "Structured Response Contract")
	assert.Contains(t, backend.requests[0].Prompt, "Required JSON schema")
}

func TestRunStructuredSpecReviewOrchestra_DowngradesMalformedOutput(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"claude": {Output: "based on what I reviewed so far, here are a few findings"},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func() orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "claude", Binary: "claude"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	require.Len(t, result.FailedProviders, 1)

	parser := &orchestra.OutputParser{}
	out, parseErr := parser.ParseReviewer(result.Responses[0].Output)
	require.NoError(t, parseErr)
	assert.Equal(t, "REVISE", out.Verdict)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "completeness", out.Findings[0].Category)
	assert.Contains(t, out.Findings[0].Description, "invalid reviewer JSON")
}
