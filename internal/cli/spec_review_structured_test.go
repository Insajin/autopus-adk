package cli

import (
	"context"
	"errors"
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
	errors   map[string]error
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
	if f.errors != nil && f.errors[req.Provider] != nil {
		return &resp, f.errors[req.Provider]
	}
	return &resp, nil
}

func (f *fakeStructuredReviewBackend) Name() string {
	return "fake-structured-review"
}

type concurrencyTrackingPaneBackend struct {
	mu        sync.Mutex
	active    int
	maxActive int
	delay     time.Duration
}

func (b *concurrencyTrackingPaneBackend) Execute(ctx context.Context, req orchestra.ProviderRequest) (*orchestra.ProviderResponse, error) {
	b.mu.Lock()
	b.active++
	if b.active > b.maxActive {
		b.maxActive = b.active
	}
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(b.delay):
	}

	b.mu.Lock()
	b.active--
	b.mu.Unlock()

	return &orchestra.ProviderResponse{
		Provider: req.Provider,
		Output:   `{"verdict":"PASS","summary":"ok","findings":[]}`,
	}, nil
}

func (b *concurrencyTrackingPaneBackend) Name() string {
	return "pane"
}

func (b *concurrencyTrackingPaneBackend) MaxActive() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxActive
}

func TestRunStructuredSpecReviewOrchestra_InjectsReviewerContract(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"claude": {Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
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

func TestRunStructuredSpecReviewOrchestra_SerializesPaneBackendExecution(t *testing.T) {
	backend := &concurrencyTrackingPaneBackend{delay: 20 * time.Millisecond}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers: []orchestra.ProviderConfig{
			{Name: "claude", Binary: "claude"},
			{Name: "codex", Binary: "codex"},
			{Name: "gemini", Binary: "agy"},
			{Name: "opencode", Binary: "opencode"},
		},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 4)
	assert.Empty(t, result.FailedProviders)
	assert.Equal(t, 1, backend.MaxActive(), "pane backend terminal I/O must not overlap")
}

func TestBuildStructuredSpecReviewPrompt_UsesScopedContractInVerifyMode(t *testing.T) {
	prompt := buildStructuredSpecReviewPrompt("### Instructions (Verify Mode)\n\nFor each finding below, report its current status.", "{}", false)

	assert.Contains(t, prompt, "In verify mode, scope the review to the prior findings")
	assert.Contains(t, prompt, "Do not perform a fresh full-SPEC discovery pass")
	assert.NotContains(t, prompt, "Review the full SPEC in one pass")
}

func TestBuildStructuredSpecReviewPrompt_UsesFullDiscoveryContractInDiscoverMode(t *testing.T) {
	prompt := buildStructuredSpecReviewPrompt("### Instructions\n\nReview the SPEC and respond with:", "{}", false)

	assert.Contains(t, prompt, "Review the full SPEC in one pass")
	assert.NotContains(t, prompt, "Do not perform a fresh full-SPEC discovery pass")
}

func TestRunStructuredSpecReviewOrchestra_DowngradesMalformedOutput(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"claude": {Output: "based on what I reviewed so far, here are a few findings"},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
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

func TestRunStructuredSpecReviewOrchestra_RecordsTimeoutDiagnostics(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"codex": {
				TimedOut: true,
				Duration: 90 * time.Second,
				Output:   "partial stdout",
				Error:    "partial stderr",
			},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "codex", Binary: "codex"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 90,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	require.Len(t, result.FailedProviders, 1)
	assert.Equal(t, "timeout", result.FailedProviders[0].FailureClass)
	assert.Contains(t, result.FailedProviders[0].Error, "provider timed out after 1m30s")
	assert.Contains(t, result.FailedProviders[0].NextRemediation, "orchestra.providers.codex.subprocess.timeout")
	assert.Equal(t, "partial stderr", result.FailedProviders[0].StderrPreview)
	assert.Equal(t, "partial stdout", result.FailedProviders[0].OutputPreview)

	parser := &orchestra.OutputParser{}
	out, parseErr := parser.ParseReviewer(result.Responses[0].Output)
	require.NoError(t, parseErr)
	require.Len(t, out.Findings, 1)
	assert.Contains(t, out.Findings[0].Description, "provider timed out after 1m30s")
	assert.Contains(t, out.Findings[0].Suggestion, "orchestra.providers.codex.subprocess.timeout")
}

func TestRunStructuredSpecReviewOrchestra_PreservesExecutionDiagnostics(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"codex": {
				Output:   "transcript stdout",
				Error:    "stderr with cli failure",
				Duration: 3 * time.Second,
				ExitCode: 1,
			},
		},
		errors: map[string]error{
			"codex": errors.New("subprocess codex exited with code 1"),
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "codex", Binary: "codex"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.FailedProviders, 1)
	assert.Equal(t, "execution_error", result.FailedProviders[0].FailureClass)
	assert.Contains(t, result.FailedProviders[0].Error, "execution failed")
	assert.Equal(t, "stderr with cli failure", result.FailedProviders[0].StderrPreview)
	assert.Equal(t, "transcript stdout", result.FailedProviders[0].OutputPreview)
}

func TestRunStructuredSpecReviewOrchestra_ClassifiesEmptyOutput(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		outputs: map[string]orchestra.ProviderResponse{
			"codex": {
				EmptyOutput: true,
				Error:       "failed to initialize in-process runtime",
				Duration:    2 * time.Second,
			},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "codex", Binary: "codex"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.FailedProviders, 1)
	assert.Equal(t, "empty_output", result.FailedProviders[0].FailureClass)
	assert.Contains(t, result.FailedProviders[0].NextRemediation, "prompt transport")
}
