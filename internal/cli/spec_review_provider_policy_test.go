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

type issue59ReviewBackend struct {
	mu        sync.Mutex
	name      string
	responses []orchestra.ProviderResponse
	requests  []orchestra.ProviderRequest
	err       error
}

func (b *issue59ReviewBackend) Execute(_ context.Context, req orchestra.ProviderRequest) (*orchestra.ProviderResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.requests = append(b.requests, req)
	index := len(b.requests) - 1
	if index >= len(b.responses) {
		index = len(b.responses) - 1
	}
	if index < 0 {
		return nil, b.err
	}
	response := b.responses[index]
	response.Provider = req.Provider
	return &response, b.err
}

func (b *issue59ReviewBackend) Name() string { return b.name }

func (b *issue59ReviewBackend) requestCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.requests)
}

func TestExecuteStructuredSpecReviewProvider_CodexExecKeepsPrimaryPane(t *testing.T) {
	pane := &issue59ReviewBackend{
		name:      "pane",
		responses: []orchestra.ProviderResponse{{Output: `{"verdict":"PASS","summary":"ok","findings":[]}`}},
	}

	outcome := executeStructuredSpecReviewProvider(
		context.Background(),
		orchestra.OrchestraConfig{TimeoutSeconds: 90, Prompt: "review"},
		pane,
		&orchestra.OutputParser{},
		"schema.json",
		"{}",
		orchestra.ProviderConfig{Name: "codex", Binary: "codex", Args: []string{"", "exec"}},
		"parallel",
	)

	require.Nil(t, outcome.failed)
	assert.Equal(t, "pane", outcome.resp.ExecutedBackend)
	assert.Equal(t, 1, pane.requestCount())
}

func TestExecuteStructuredSpecReviewProvider_CodexRepromptKeepsPrimaryPane(t *testing.T) {
	pane := &issue59ReviewBackend{
		name: "pane",
		responses: []orchestra.ProviderResponse{
			{Output: "not json"},
			{Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}
	outcome := executeStructuredSpecReviewProvider(
		context.Background(),
		orchestra.OrchestraConfig{TimeoutSeconds: 90, Prompt: "review"},
		pane,
		&orchestra.OutputParser{},
		"schema.json",
		`{"type":"object"}`,
		orchestra.ProviderConfig{
			Name:       "codex",
			Binary:     "codex",
			Args:       []string{"", "exec", "--json"},
			SchemaFlag: "--output-schema",
		},
		"parallel",
	)

	require.Nil(t, outcome.failed)
	assert.Equal(t, "pane", outcome.resp.ExecutedBackend)
	require.Equal(t, 2, pane.requestCount(), "initial request and reprompt must use the primary pane backend")
	pane.mu.Lock()
	defer pane.mu.Unlock()
	assert.Contains(t, pane.requests[0].Prompt, "Required JSON schema:")
	assert.Contains(t, pane.requests[1].Prompt, "Required JSON schema:")
}

func TestSpecReviewProviderBackendName_UsesPrimaryBackend(t *testing.T) {
	pane := &issue59ReviewBackend{name: "pane"}
	subprocess := &issue59ReviewBackend{name: "subprocess"}
	tests := []struct {
		name    string
		backend orchestra.ExecutionBackend
		want    string
	}{
		{name: "pane", backend: pane, want: "pane"},
		{name: "explicit subprocess", backend: subprocess, want: "subprocess"},
		{name: "missing", backend: nil, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, specReviewProviderBackendName(tt.backend))
		})
	}
}

func TestExecuteStructuredSpecReviewProvider_UsesSelectedBackendForFailureDiagnostics(t *testing.T) {
	pane := &issue59ReviewBackend{name: "pane", err: errors.New("boom")}

	outcome := executeStructuredSpecReviewProvider(
		context.Background(),
		orchestra.OrchestraConfig{TimeoutSeconds: 90, Prompt: "review"},
		pane,
		&orchestra.OutputParser{},
		"schema.json",
		"{}",
		orchestra.ProviderConfig{Name: "codex", Binary: "codex", Args: []string{"exec"}},
		"parallel",
	)

	require.NotNil(t, outcome.failed)
	assert.Equal(t, "pane", outcome.failed.CollectionMode)
	assert.Equal(t, "pane", outcome.resp.ExecutedBackend)
	assert.Equal(t, 1, pane.requestCount())
}

func TestApplySpecReviewExecutionTimeout_PreservesDefaultsAndInput(t *testing.T) {
	input := []orchestra.ProviderConfig{
		{Name: "codex", ExecutionTimeout: 420 * time.Second},
		{Name: "claude", ExecutionTimeout: 480 * time.Second},
		{Name: "gemini", ExecutionTimeout: 480 * time.Second},
	}
	wantInput := append([]orchestra.ProviderConfig(nil), input...)

	got := applySpecReviewExecutionTimeout(input, 0)

	assert.Equal(t, wantInput, got)
	assert.Equal(t, wantInput, input)
	assert.NotSame(t, &input[0], &got[0])
}

func TestApplySpecReviewExecutionTimeout_OverridesAllBudgetsWithoutMutation(t *testing.T) {
	input := []orchestra.ProviderConfig{
		{Name: "codex", ExecutionTimeout: 420 * time.Second},
		{Name: "claude", ExecutionTimeout: 480 * time.Second},
		{Name: "gemini", ExecutionTimeout: 480 * time.Second},
	}
	wantInput := append([]orchestra.ProviderConfig(nil), input...)

	got := applySpecReviewExecutionTimeout(input, 90)

	for _, provider := range got {
		assert.Equal(t, 90*time.Second, provider.ExecutionTimeout)
		assert.Equal(t, 180*time.Second, specReviewAttemptTimeoutBudget(provider, 240))
	}
	assert.Equal(t, 240, specReviewWatchdogSeconds(got, 240))
	assert.Equal(t, wantInput, input)
}
