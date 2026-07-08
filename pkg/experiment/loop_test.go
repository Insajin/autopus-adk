package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoopRun_MaxIterationsStopsBeforeSixth(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxIterations = 5
	cfg.CircuitBreakerN = 50
	cfg.ExperimentTimeout = 0
	loop := NewLoop(cfg)

	calls := 0
	summary, reason, err := loop.Run(context.Background(), func(ctx context.Context, iteration int) (StepResult, error) {
		calls++
		return noImprovementStepResult(iteration), nil
	})

	require.NoError(t, err)
	assert.Equal(t, 5, calls)
	assert.Equal(t, StopReasonMaxIterations, reason)
	assert.Equal(t, 5, summary.TotalIterations)
}

func TestLoopRun_CircuitBreakerStopsAndResetsOnImprovement(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxIterations = 50
	cfg.CircuitBreakerN = 3
	cfg.ExperimentTimeout = 0

	noProgressCalls := 0
	summary, reason, err := NewLoop(cfg).Run(context.Background(), func(ctx context.Context, iteration int) (StepResult, error) {
		noProgressCalls++
		return noImprovementStepResult(iteration), nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, noProgressCalls)
	assert.Equal(t, StopReasonCircuitBreaker, reason)
	assert.Equal(t, 3, summary.TotalIterations)

	resetCalls := 0
	summary, reason, err = NewLoop(cfg).Run(context.Background(), func(ctx context.Context, iteration int) (StepResult, error) {
		resetCalls++
		if iteration == 2 {
			return StepResult{
				Result: Result{
					Iteration:   iteration,
					MetricValue: float64(iteration),
					Status:      "keep",
					Timestamp:   time.Now(),
				},
				Improved: true,
			}, nil
		}
		return noImprovementStepResult(iteration), nil
	})

	require.NoError(t, err)
	assert.Greater(t, resetCalls, 3)
	assert.Equal(t, StopReasonCircuitBreaker, reason)
	assert.Equal(t, resetCalls, summary.TotalIterations)
}

func TestLoopRun_ContextCancellationStopsAfterFirstStep(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxIterations = 50
	cfg.CircuitBreakerN = 50
	cfg.ExperimentTimeout = 0
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := 0
	summary, reason, err := NewLoop(cfg).Run(ctx, func(ctx context.Context, iteration int) (StepResult, error) {
		calls++
		cancel()
		return noImprovementStepResult(iteration), nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, StopReasonCancelled, reason)
	assert.Equal(t, 1, summary.TotalIterations)
}

func TestLoopRun_TimeoutStopsAfterFirstStep(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxIterations = 50
	cfg.CircuitBreakerN = 50
	cfg.ExperimentTimeout = time.Millisecond

	calls := 0
	summary, reason, err := NewLoop(cfg).Run(context.Background(), func(ctx context.Context, iteration int) (StepResult, error) {
		calls++
		<-ctx.Done()
		return noImprovementStepResult(iteration), nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, StopReasonTimeout, reason)
	assert.Equal(t, 1, summary.TotalIterations)
}

func noImprovementStepResult(iteration int) StepResult {
	return StepResult{
		Result: Result{
			Iteration:   iteration,
			MetricValue: float64(iteration),
			Status:      "discard",
			Timestamp:   time.Now(),
		},
		Improved: false,
	}
}
