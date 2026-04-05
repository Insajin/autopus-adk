// Package pipeline provides pipeline state management types and persistence.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// RunConfig holds optional configuration for a runner execution.
type RunConfig struct {
	// SpecID is used to name the checkpoint file when CheckpointDir is set.
	SpecID string
	// CheckpointDir is the directory where checkpoint files are written.
	// If empty, no checkpoint is saved.
	CheckpointDir string
}

// SequentialRunner executes pipeline phases one at a time in order.
type SequentialRunner struct {
	backend PhaseBackend
}

// NewSequentialRunner creates a SequentialRunner backed by the given backend.
func NewSequentialRunner(backend PhaseBackend) *SequentialRunner {
	return &SequentialRunner{backend: backend}
}

// RunPhases executes the given phases sequentially and returns their results.
// When a phase gate fails it retries up to Phase.MaxRetries times.
// An error is returned if max retries are exceeded.
func (r *SequentialRunner) RunPhases(ctx context.Context, phases []Phase, cfg RunConfig) ([]PhaseResult, error) {
	results := make([]PhaseResult, 0, len(phases))
	var previousOutput string

	for _, phase := range phases {
		result, err := r.runPhaseWithRetry(ctx, phase, previousOutput)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
		previousOutput = result.Output

		if cfg.CheckpointDir != "" && cfg.SpecID != "" {
			if saveErr := saveRunCheckpoint(cfg, phase.ID, results); saveErr != nil {
				// Checkpoint save failure is non-fatal — log and continue.
				_ = saveErr
			}
		}
	}

	return results, nil
}

// defaultMaxRetries is the safety cap when Phase.MaxRetries is 0.
const defaultMaxRetries = 10

// runPhaseWithRetry executes a single phase, retrying on gate failure.
// When MaxRetries is 0, a safety cap of defaultMaxRetries is applied.
func (r *SequentialRunner) runPhaseWithRetry(ctx context.Context, phase Phase, previousOutput string) (PhaseResult, error) {
	prompt := buildRunnerPrompt(phase.ID, previousOutput)

	maxRetries := phase.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	for attempt := 0; ; attempt++ {
		resp, err := r.backend.Execute(ctx, PhaseRequest{Prompt: prompt, PhaseID: phase.ID})
		if err != nil {
			return PhaseResult{}, fmt.Errorf("phase %s: %w", phase.ID, err)
		}

		verdict := EvaluateGate(phase.Gate, resp.Output)
		if verdict == VerdictPass || phase.Gate == GateNone {
			return PhaseResult{PhaseID: phase.ID, Output: resp.Output, Verdict: verdict}, nil
		}

		if attempt >= maxRetries {
			return PhaseResult{}, fmt.Errorf("phase %s: max retries (%d) exceeded", phase.ID, maxRetries)
		}
	}
}

// ParallelRunner executes pipeline phases concurrently.
type ParallelRunner struct {
	backend PhaseBackend
}

// NewParallelRunner creates a ParallelRunner backed by the given backend.
func NewParallelRunner(backend PhaseBackend) *ParallelRunner {
	return &ParallelRunner{backend: backend}
}

// @AX:WARN: [AUTO] goroutines without context cancellation check in worker body — ctx is passed to Execute but goroutine does not short-circuit on ctx.Done() before calling Execute
// RunPhases executes all given phases in parallel and returns their results.
// Results are returned in the same order as the input phases.
func (r *ParallelRunner) RunPhases(ctx context.Context, phases []Phase, _ RunConfig) ([]PhaseResult, error) {
	n := len(phases)
	results := make([]PhaseResult, n)
	errs := make([]error, n)

	// @AX:NOTE: [AUTO] start-gun pattern — gate channel releases all goroutines simultaneously; maximizes concurrency burst
	// gate is closed after all goroutines are launched, releasing them
	// simultaneously to maximise observable concurrency.
	gate := make(chan struct{})

	var wg sync.WaitGroup
	for i, phase := range phases {
		wg.Add(1)
		go func(idx int, ph Phase) {
			defer wg.Done()
			<-gate // wait for all goroutines to be launched before executing
			resp, err := r.backend.Execute(ctx, PhaseRequest{PhaseID: ph.ID})
			if err != nil {
				errs[idx] = fmt.Errorf("phase %s: %w", ph.ID, err)
				return
			}
			verdict := EvaluateGate(ph.Gate, resp.Output)
			results[idx] = PhaseResult{PhaseID: ph.ID, Output: resp.Output, Verdict: verdict}
		}(i, phase)
	}
	close(gate) // release all goroutines at once
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// buildRunnerPrompt constructs a phase prompt injecting the previous output.
func buildRunnerPrompt(phaseID PhaseID, previousOutput string) string {
	if previousOutput == "" {
		return fmt.Sprintf("Phase: %s", phaseID)
	}
	return fmt.Sprintf("Phase: %s\n\nPrevious output:\n%s", phaseID, previousOutput)
}

// saveRunCheckpoint writes a checkpoint file to cfg.CheckpointDir/{SpecID}.yaml.
func saveRunCheckpoint(cfg RunConfig, lastPhase PhaseID, results []PhaseResult) error {
	taskStatus := make(map[string]string, len(results))
	for _, r := range results {
		taskStatus[string(r.PhaseID)] = string(CheckpointStatusDone)
	}

	data, err := yaml.Marshal(map[string]interface{}{
		"phase":       string(lastPhase),
		"task_status": taskStatus,
	})
	if err != nil {
		return err
	}

	path := filepath.Join(cfg.CheckpointDir, cfg.SpecID+".yaml")
	return os.WriteFile(path, data, 0o644)
}
