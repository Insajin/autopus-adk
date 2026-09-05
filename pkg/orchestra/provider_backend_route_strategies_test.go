package orchestra

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every strategy that used to call runProvider directly must honour a
// configured backend; an OMP provider with no installed binary is the oracle.
func TestRunOrchestra_EveryStrategyRoutesConfiguredBackend(t *testing.T) {
	for _, strategy := range []Strategy{StrategyFastest, StrategyPipeline, StrategyRelay, StrategyRecheck} {
		t.Run(string(strategy), func(t *testing.T) {
			backend := &configuredProviderBackendFake{response: ProviderResponse{Output: "routed " + string(strategy)}}
			provider := ProviderConfig{
				Name: "reviewer", Backend: "omp", Binary: "missing-provider-binary",
			}

			result, err := RunOrchestra(context.Background(), OrchestraConfig{
				Providers:        []ProviderConfig{provider},
				ProviderBackends: map[string]ExecutionBackend{"omp": backend},
				Strategy:         strategy,
				Prompt:           "topic",
				TimeoutSeconds:   10,
			})

			require.NoError(t, err)
			require.NotEmpty(t, backend.requests, "strategy %s never reached the configured backend", strategy)
			require.NotEmpty(t, result.Responses)
			assert.Equal(t, "omp", result.Responses[0].ExecutedBackend)
		})
	}
}

// Without a registered backend an OMP provider must fail closed on every
// strategy instead of falling back to the missing binary.
func TestRunOrchestra_EveryStrategyFailsClosedWithoutBackend(t *testing.T) {
	for _, strategy := range []Strategy{StrategyFastest, StrategyPipeline, StrategyRelay, StrategyRecheck} {
		t.Run(string(strategy), func(t *testing.T) {
			provider := ProviderConfig{Name: "reviewer", Backend: "omp", Binary: "missing-provider-binary"}

			result, err := RunOrchestra(context.Background(), OrchestraConfig{
				Providers: []ProviderConfig{provider}, Strategy: strategy, Prompt: "topic", TimeoutSeconds: 10,
			})

			// fastest/relay fold the per-provider error into an aggregate
			// message; the invariant is that nothing produced a response.
			if err == nil {
				require.NotNil(t, result)
			}
			assert.True(t, err != nil || len(result.Responses) == 0,
				"strategy %s produced a response without a backend", strategy)
		})
	}
}
