package orchestra

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type configuredProviderBackendFake struct {
	mu       sync.Mutex
	requests []ProviderRequest
	response ProviderResponse
	err      error
}

func (f *configuredProviderBackendFake) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	response := f.response
	return &response, f.err
}

func (*configuredProviderBackendFake) Name() string { return "omp" }

func (*configuredProviderBackendFake) FreshExecutionPerRequest() bool { return true }

func (f *configuredProviderBackendFake) lastRequest(t *testing.T) ProviderRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.requests)
	return f.requests[len(f.requests)-1]
}

func TestRunConfiguredProvider_RoutesRequestAndPreservesBackendEvidence(t *testing.T) {
	backend := &configuredProviderBackendFake{response: ProviderResponse{
		Output:          "routed output",
		ExecutedBackend: "observed-omp",
	}}
	provider := ProviderConfig{
		Name: "reviewer", Backend: "omp", Binary: "missing-provider-binary",
		ExecutionTimeout: 3 * time.Second,
	}
	tracker := NewProgressTracker([]string{provider.Name})

	response, err := runConfiguredProvider(
		context.Background(),
		OrchestraConfig{TimeoutSeconds: 9, ProviderBackends: map[string]ExecutionBackend{"omp": backend}},
		provider,
		"review prompt",
		"reviewer",
		2,
		tracker,
	)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "observed-omp", response.ExecutedBackend)
	assert.Equal(t, "reviewer", response.Provider)
	assert.Equal(t, "reviewer", response.Role)
	assert.Equal(t, 2, response.Attempt)
	request := backend.lastRequest(t)
	assert.Equal(t, "review prompt", request.Prompt)
	assert.Equal(t, "reviewer", request.Role)
	assert.Equal(t, 2, request.Round)
	assert.Equal(t, 3*time.Second, request.Timeout)
	assert.Equal(t, provider, request.Config)
	assert.Equal(t, StatusDone, tracker.providers[provider.Name].status)
}

func TestRunConfiguredProvider_BackendFailureMarksProgressFailed(t *testing.T) {
	backend := &configuredProviderBackendFake{err: errors.New("route failed")}
	provider := ProviderConfig{Name: "reviewer", Backend: "omp"}
	tracker := NewProgressTracker([]string{provider.Name})

	_, err := runConfiguredProvider(
		context.Background(),
		OrchestraConfig{ProviderBackends: map[string]ExecutionBackend{"omp": backend}},
		provider,
		"prompt",
		"participant",
		1,
		tracker,
	)

	require.EqualError(t, err, "route failed")
	assert.Equal(t, StatusFailed, tracker.providers[provider.Name].status)
}

func TestRunConfiguredProvider_UnregisteredBackendFailsClosed(t *testing.T) {
	provider := ProviderConfig{Name: "reviewer", Backend: "omp"}

	response, err := runConfiguredProvider(
		context.Background(), OrchestraConfig{}, provider, "prompt", "participant", 1, nil,
	)

	assert.Nil(t, response)
	require.EqualError(t, err, `provider reviewer backend "omp" is not available in this execution path`)
}

func TestRunOrchestra_OMPProviderSkipsBinaryPreflightAndRoutesBackend(t *testing.T) {
	backend := &configuredProviderBackendFake{response: ProviderResponse{Output: "routed consensus"}}
	provider := ProviderConfig{
		Name: "reviewer", Backend: "omp", Binary: "missing-provider-binary",
	}

	result, err := RunOrchestra(context.Background(), OrchestraConfig{
		Providers:        []ProviderConfig{provider},
		ProviderBackends: map[string]ExecutionBackend{"omp": backend},
		Strategy:         StrategyConsensus,
		Prompt:           "topic",
		TimeoutSeconds:   10,
	})

	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	assert.Equal(t, "omp", result.Responses[0].ExecutedBackend)
	assert.Equal(t, "participant", backend.lastRequest(t).Role)
}

func TestRunRebuttalRound_RoutesConfiguredBackend(t *testing.T) {
	backend := &configuredProviderBackendFake{response: ProviderResponse{Output: "routed rebuttal"}}
	provider := ProviderConfig{Name: "debater", Backend: "omp", Binary: "missing-provider-binary"}
	cfg := OrchestraConfig{
		Providers:        []ProviderConfig{provider},
		ProviderBackends: map[string]ExecutionBackend{"omp": backend},
		Prompt:           "topic",
		TimeoutSeconds:   10,
	}

	responses, failed, err := runRebuttalRound(context.Background(), cfg, []ProviderResponse{
		{Provider: "peer", Output: "peer argument"},
	})

	require.NoError(t, err)
	assert.Empty(t, failed)
	require.Len(t, responses, 1)
	assert.Equal(t, "omp", responses[0].ExecutedBackend)
	request := backend.lastRequest(t)
	assert.Equal(t, "debater_r2", request.Role)
	assert.Equal(t, 2, request.Round)
}

func TestExecuteDebateJudge_RoutesConfiguredBackend(t *testing.T) {
	backend := &configuredProviderBackendFake{response: ProviderResponse{
		Output:          `{"recommendation":"ship the routed result"}`,
		ExecutedBackend: "omp",
	}}
	judge := ProviderConfig{Name: "judge", Backend: "omp", Binary: "missing-judge-binary"}
	cfg := OrchestraConfig{
		Providers:        []ProviderConfig{{Name: "debater", Binary: "unused"}},
		ProviderBackends: map[string]ExecutionBackend{"omp": backend},
		Prompt:           "topic",
		TimeoutSeconds:   10,
		JudgeProvider:    judge.Name,
		JudgeConfig:      &judge,
		DebateRounds:     2,
	}

	response, failure, evidence := executeDebateJudge(context.Background(), cfg, []ProviderResponse{
		{Provider: "debater", Output: "argument"},
	})

	assert.Nil(t, failure)
	require.NotNil(t, response)
	assert.Equal(t, "judge (judge)", response.Provider)
	assert.Equal(t, "omp", response.ExecutedBackend)
	require.NotNil(t, evidence)
	assert.True(t, evidence.Verified)
	request := backend.lastRequest(t)
	assert.Equal(t, "judge", request.Role)
	assert.Equal(t, 3, request.Round)
}
