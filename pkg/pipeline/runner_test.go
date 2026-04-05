package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// TestSequentialRunner_RunPhases_ExecutesInOrder verifies that
// SequentialRunner executes phases in the canonical order (REQ-4).
func TestSequentialRunner_RunPhases_ExecutesInOrder(t *testing.T) {
	t.Parallel()

	// Given: a SequentialRunner with 5 phases and a recording backend
	recorder := &FakeBackend{
		Responses: []string{"out1", "out2", "out3", "out4", "out5"},
	}
	phases := pipeline.DefaultPhases()
	runner := pipeline.NewSequentialRunner(recorder)

	// When: RunPhases is called
	results, err := runner.RunPhases(context.Background(), phases, pipeline.RunConfig{})

	// Then: phases are executed in order and all results are present
	require.NoError(t, err)
	assert.Len(t, results, 5)

	ids := make([]pipeline.PhaseID, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.PhaseID)
	}
	assert.Equal(t, pipeline.PhasePlan, ids[0])
	assert.Equal(t, pipeline.PhaseTestScaffold, ids[1])
	assert.Equal(t, pipeline.PhaseImplement, ids[2])
	assert.Equal(t, pipeline.PhaseValidate, ids[3])
	assert.Equal(t, pipeline.PhaseReview, ids[4])
}

// TestSequentialRunner_RunPhases_SavesCheckpoint verifies that
// SequentialRunner saves a checkpoint after each phase (REQ-7).
func TestSequentialRunner_RunPhases_SavesCheckpoint(t *testing.T) {
	t.Parallel()

	// Given: a SequentialRunner with a checkpoint directory
	dir := t.TempDir()
	recorder := &FakeBackend{
		Responses: []string{"out1", "out2", "out3", "out4", "out5"},
	}
	phases := pipeline.DefaultPhases()
	runner := pipeline.NewSequentialRunner(recorder)
	cfg := pipeline.RunConfig{
		CheckpointDir: dir,
		SpecID:        "SPEC-RUN-001",
	}

	// When: RunPhases is called
	_, err := runner.RunPhases(context.Background(), phases, cfg)

	// Then: a checkpoint file exists in the dir
	require.NoError(t, err)
	cpPath := filepath.Join(dir, "SPEC-RUN-001.yaml")
	_, statErr := os.Stat(cpPath)
	assert.NoError(t, statErr)
}

// TestSequentialRunner_RunPhases_GateFail_Retries verifies that
// SequentialRunner retries a phase when the gate verdict is Fail (REQ-6).
func TestSequentialRunner_RunPhases_GateFail_Retries(t *testing.T) {
	t.Parallel()

	// Given: a backend that returns FAIL on first call, PASS on second
	recorder := &FakeBackend{
		Responses: []string{
			"FAIL: first attempt",  // validate phase fails
			"PASS: second attempt", // retry succeeds
			"out3", "out4", "out5",
		},
	}
	phases := []pipeline.Phase{
		{ID: pipeline.PhaseValidate, Gate: pipeline.GateValidation},
	}
	runner := pipeline.NewSequentialRunner(recorder)

	// When: RunPhases is called
	results, err := runner.RunPhases(context.Background(), phases, pipeline.RunConfig{})

	// Then: the phase is retried and eventually passes
	require.NoError(t, err)
	assert.True(t, recorder.CallCount >= 2)
	require.Len(t, results, 1)
	assert.Equal(t, pipeline.VerdictPass, results[0].Verdict)
}

// TestSequentialRunner_RunPhases_GateFail_ExceedsRetry verifies that
// SequentialRunner returns an error when max retries are exceeded (REQ-6).
func TestSequentialRunner_RunPhases_GateFail_ExceedsRetry(t *testing.T) {
	t.Parallel()

	// Given: a backend that always returns FAIL
	recorder := &FakeBackend{
		// 4 responses to cover 1 initial + 3 retries
		Responses: []string{"FAIL", "FAIL", "FAIL", "FAIL"},
	}
	phases := []pipeline.Phase{
		{ID: pipeline.PhaseValidate, Gate: pipeline.GateValidation, MaxRetries: 3},
	}
	runner := pipeline.NewSequentialRunner(recorder)

	// When: RunPhases is called
	_, err := runner.RunPhases(context.Background(), phases, pipeline.RunConfig{})

	// Then: an error is returned indicating max retries exceeded
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
}

// TestParallelRunner_RunPhases_ParallelExecution verifies that ParallelRunner
// executes independent phases concurrently (REQ-5).
func TestParallelRunner_RunPhases_ParallelExecution(t *testing.T) {
	t.Parallel()

	// Given: a ParallelRunner with independent phases and a concurrent recorder
	recorder := &FakeConcurrentBackend{
		Responses: map[pipeline.PhaseID]string{
			pipeline.PhasePlan:         "plan out",
			pipeline.PhaseTestScaffold: "test out",
		},
	}
	phases := []pipeline.Phase{
		{ID: pipeline.PhasePlan, Gate: pipeline.GateNone},
		{ID: pipeline.PhaseTestScaffold, Gate: pipeline.GateNone},
	}
	runner := pipeline.NewParallelRunner(recorder)

	// When: RunPhases is called
	results, err := runner.RunPhases(context.Background(), phases, pipeline.RunConfig{})

	// Then: both phases are executed and results contain both
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, recorder.MaxConcurrent >= 2,
		"expected at least 2 concurrent executions, got %d", recorder.MaxConcurrent)
}
