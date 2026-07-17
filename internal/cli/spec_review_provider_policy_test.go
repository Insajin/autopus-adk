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

func TestExecuteStructuredSpecReviewProvider_CodexExecCallsOnlySubprocess(t *testing.T) {
	pane := &issue59ReviewBackend{name: "pane"}
	subprocess := &issue59ReviewBackend{
		name:      "subprocess",
		responses: []orchestra.ProviderResponse{{Output: `{"verdict":"PASS","summary":"ok","findings":[]}`}},
	}
	restore := stubSpecReviewSubprocessBackend(t, subprocess)
	defer restore()

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
	assert.Equal(t, "subprocess", outcome.resp.ExecutedBackend)
	assert.Zero(t, pane.requestCount())
	assert.Equal(t, 1, subprocess.requestCount())
}

func TestExecuteStructuredSpecReviewProvider_PreselectsSubprocessForCodexReprompt(t *testing.T) {
	pane := &issue59ReviewBackend{name: "pane"}
	subprocess := &issue59ReviewBackend{
		name: "subprocess",
		responses: []orchestra.ProviderResponse{
			{Output: "not json"},
			{Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}
	restore := stubSpecReviewSubprocessBackend(t, subprocess)
	defer restore()

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
	assert.Equal(t, "subprocess", outcome.resp.ExecutedBackend)
	assert.Zero(t, pane.requestCount())
	require.Equal(t, 2, subprocess.requestCount(), "reprompt must use the selected subprocess backend")
	subprocess.mu.Lock()
	defer subprocess.mu.Unlock()
	assert.NotContains(t, subprocess.requests[0].Prompt, "Required JSON schema:")
	assert.NotContains(t, subprocess.requests[1].Prompt, "Required JSON schema:")
}

func TestSelectSpecReviewProviderBackend_PreservesNonEligibleBackends(t *testing.T) {
	pane := &issue59ReviewBackend{name: "pane"}
	subprocess := &issue59ReviewBackend{name: "subprocess"}
	created := 0
	restore := stubSpecReviewSubprocessFactory(t, func() orchestra.ExecutionBackend {
		created++
		return subprocess
	})
	defer restore()

	tests := []struct {
		name     string
		primary  orchestra.ExecutionBackend
		provider orchestra.ProviderConfig
	}{
		{name: "claude", primary: pane, provider: orchestra.ProviderConfig{Name: "claude", Binary: "claude", Args: []string{"--print"}}},
		{name: "gemini", primary: pane, provider: orchestra.ProviderConfig{Name: "gemini", Binary: "gemini", Args: []string{"exec"}}},
		{name: "codex non-exec", primary: pane, provider: orchestra.ProviderConfig{Name: "codex", Binary: "codex", Args: []string{"--search", "exec"}}},
		{name: "codex case mismatch", primary: pane, provider: orchestra.ProviderConfig{Name: "codex", Binary: "codex", Args: []string{"Exec"}}},
		{name: "forced subprocess", primary: subprocess, provider: orchestra.ProviderConfig{Name: "codex", Binary: "codex", Args: []string{"exec"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := selectSpecReviewProviderBackend(tt.primary, tt.provider)
			assert.Same(t, tt.primary, selected)
		})
	}
	assert.Zero(t, created)
}

func TestExecuteStructuredSpecReviewProvider_UsesSelectedBackendForFailureDiagnostics(t *testing.T) {
	pane := &issue59ReviewBackend{name: "pane"}
	subprocess := &issue59ReviewBackend{name: "subprocess", err: errors.New("boom")}
	restore := stubSpecReviewSubprocessBackend(t, subprocess)
	defer restore()

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
	assert.Equal(t, "subprocess_stdout", outcome.failed.CollectionMode)
	assert.Equal(t, "subprocess", outcome.resp.ExecutedBackend)
	assert.Zero(t, pane.requestCount())
	assert.Equal(t, 1, subprocess.requestCount())
}

func stubSpecReviewSubprocessBackend(t *testing.T, backend orchestra.ExecutionBackend) func() {
	t.Helper()
	return stubSpecReviewSubprocessFactory(t, func() orchestra.ExecutionBackend { return backend })
}

func stubSpecReviewSubprocessFactory(t *testing.T, factory func() orchestra.ExecutionBackend) func() {
	t.Helper()
	original := specReviewSubprocessBackendFactory
	specReviewSubprocessBackendFactory = factory
	return func() { specReviewSubprocessBackendFactory = original }
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
