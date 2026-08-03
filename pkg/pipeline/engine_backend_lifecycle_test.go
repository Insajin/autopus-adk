package pipeline_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

type lifecyclePhaseBackend struct {
	executeErr error
	closeErr   error
	executes   int
	closes     int
}

var _ pipeline.PhaseBackend = (*lifecyclePhaseBackend)(nil)
var _ pipeline.PhaseBackendCloser = (*lifecyclePhaseBackend)(nil)

func (backend *lifecyclePhaseBackend) Execute(
	_ context.Context,
	request pipeline.PhaseRequest,
) (*pipeline.PhaseResponse, error) {
	backend.executes++
	if backend.executeErr != nil {
		return &pipeline.PhaseResponse{}, backend.executeErr
	}
	output := "phase completed"
	switch request.PhaseID {
	case pipeline.PhaseValidate:
		output = "VERDICT: PASS"
	case pipeline.PhaseReview:
		output = "VERDICT: APPROVE"
	}
	return &pipeline.PhaseResponse{Output: output}, nil
}

func (backend *lifecyclePhaseBackend) Close() error {
	backend.closes++
	return backend.closeErr
}

func TestSubprocessEngine_RunClosesOptionalBackendExactlyOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		executeErr error
		wantErr    bool
		wantCalls  int
	}{
		{name: "success", wantCalls: len(pipeline.DefaultPhases())},
		{name: "backend failure", executeErr: errors.New("fixture backend failed"), wantErr: true, wantCalls: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			backend := &lifecyclePhaseBackend{executeErr: test.executeErr}
			engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
				SpecID: "SPEC-OMP-004", Platform: "codex",
				Strategy: pipeline.StrategySequential, Backend: backend,
			})

			result, err := engine.Run(context.Background())

			if test.wantErr {
				require.ErrorContains(t, err, test.executeErr.Error())
				require.NotNil(t, result)
				assert.Equal(t, pipeline.TerminalBlocked, result.Receipt.Terminal)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, pipeline.TerminalCompleted, result.Receipt.Terminal)
			}
			assert.Equal(t, test.wantCalls, backend.executes)
			assert.Equal(t, 1, backend.closes, "Run must own and close the optional backend lifecycle")
		})
	}
}

func TestSubprocessEngine_RunPreservesPrimaryAndCleanupFailures(t *testing.T) {
	t.Parallel()

	cleanupErr := errors.New("fixture cleanup failed")
	t.Run("completed run becomes partial preserved", func(t *testing.T) {
		backend := &lifecyclePhaseBackend{closeErr: cleanupErr}
		result, err := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
			SpecID: "SPEC-OMP-004", Platform: "codex",
			Strategy: pipeline.StrategySequential, Backend: backend,
		}).Run(context.Background())

		require.ErrorContains(t, err, cleanupErr.Error())
		require.NotNil(t, result)
		assert.Equal(t, pipeline.TerminalPartialPreserved, result.Receipt.Terminal)
		assert.Contains(t, result.Receipt.DegradedReasons, "backend_cleanup_failure")
		assert.Equal(t, 1, backend.closes)
	})

	t.Run("primary failure remains joined", func(t *testing.T) {
		primaryErr := errors.New("fixture primary failed")
		backend := &lifecyclePhaseBackend{executeErr: primaryErr, closeErr: cleanupErr}
		result, err := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
			SpecID: "SPEC-OMP-004", Platform: "codex",
			Strategy: pipeline.StrategySequential, Backend: backend,
		}).Run(context.Background())

		require.ErrorContains(t, err, primaryErr.Error())
		require.ErrorContains(t, err, cleanupErr.Error())
		require.NotNil(t, result)
		assert.Equal(t, pipeline.TerminalBlocked, result.Receipt.Terminal)
		assert.Contains(t, result.Receipt.DegradedReasons, "backend_cleanup_failure")
		assert.Equal(t, 1, backend.closes)
	})
}
