package pipeline_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// TestSubprocessEngine_Run_ExecutesAllPhases verifies that SubprocessEngine
// executes all 5 pipeline phases sequentially (REQ-1).
func TestSubprocessEngine_Run_ExecutesAllPhases(t *testing.T) {
	t.Parallel()

	// Given: a SubprocessEngine configured for a known SPEC
	cfg := pipeline.EngineConfig{
		SpecID:   "SPEC-TEST-001",
		Platform: "codex",
		Strategy: pipeline.StrategySequential,
	}
	engine := pipeline.NewSubprocessEngine(cfg)

	// When: Run is called
	result, err := engine.Run(context.Background())

	// Then: all 5 phases are executed and reported in the result
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.PhaseResults, 5)
}

// TestSubprocessEngine_Run_InjectsResultToNextPhase verifies that each phase's
// output is injected into the next phase's prompt (REQ-2).
func TestSubprocessEngine_Run_InjectsResultToNextPhase(t *testing.T) {
	t.Parallel()

	// Given: a SubprocessEngine with a fake backend that records prompts
	recorder := &FakeBackend{
		Responses: []string{
			"plan output",
			"test scaffold output",
			"implement output",
			"validate output",
			"review output",
		},
	}
	cfg := pipeline.EngineConfig{
		SpecID:   "SPEC-TEST-001",
		Platform: "codex",
		Strategy: pipeline.StrategySequential,
		Backend:  recorder,
	}
	engine := pipeline.NewSubprocessEngine(cfg)

	// When: Run is called
	_, err := engine.Run(context.Background())

	// Then: phase 2's prompt contains phase 1's output
	require.NoError(t, err)
	require.True(t, len(recorder.ReceivedPrompts) >= 2)
	assert.Contains(t, recorder.ReceivedPrompts[1], "plan output")
}

// TestSubprocessEngine_Run_ResumeFromCheckpoint verifies that when a checkpoint
// exists, the engine skips already-completed phases (REQ-7).
func TestSubprocessEngine_Run_ResumeFromCheckpoint(t *testing.T) {
	t.Parallel()

	// Given: a checkpoint indicating phase 1 (Plan) is done
	cp := &pipeline.Checkpoint{
		Phase:         "plan",
		GitCommitHash: "abc123",
		TaskStatus: map[string]pipeline.CheckpointStatus{
			"plan": pipeline.CheckpointStatusDone,
		},
	}
	recorder := &FakeBackend{
		Responses: []string{
			"test scaffold output",
			"implement output",
			"validate output",
			"review output",
		},
	}
	cfg := pipeline.EngineConfig{
		SpecID:     "SPEC-TEST-001",
		Platform:   "codex",
		Strategy:   pipeline.StrategySequential,
		Backend:    recorder,
		Checkpoint: cp,
	}
	engine := pipeline.NewSubprocessEngine(cfg)

	// When: Run is called
	result, err := engine.Run(context.Background())

	// Then: only 4 phases are executed (plan was skipped)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 4, recorder.CallCount)
	assert.Len(t, result.PhaseResults, 5)
}

// TestSubprocessEngine_Run_DryRun verifies that dry-run mode does not invoke
// any subprocess.
func TestSubprocessEngine_Run_DryRun(t *testing.T) {
	t.Parallel()

	// Given: a SubprocessEngine in dry-run mode
	recorder := &FakeBackend{}
	cfg := pipeline.EngineConfig{
		SpecID:   "SPEC-TEST-001",
		Platform: "codex",
		Strategy: pipeline.StrategySequential,
		Backend:  recorder,
		DryRun:   true,
	}
	engine := pipeline.NewSubprocessEngine(cfg)

	// When: Run is called
	result, err := engine.Run(context.Background())

	// Then: no subprocess is invoked
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, recorder.CallCount)
}
