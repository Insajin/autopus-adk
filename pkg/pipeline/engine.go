// Package pipeline provides pipeline state management types and persistence.
package pipeline

import (
	"context"
	"fmt"
)

// Strategy defines the execution order of pipeline phases.
type Strategy string

const (
	// StrategySequential executes phases one after another.
	StrategySequential Strategy = "sequential"
	// StrategyParallel executes independent phases concurrently.
	StrategyParallel Strategy = "parallel"
)

// PhaseID identifies a pipeline phase.
type PhaseID string

// GateVerdict is the outcome of a phase gate evaluation.
type GateVerdict string

// PhaseBackend is the interface for executing pipeline phases.
type PhaseBackend interface {
	Execute(ctx context.Context, req PhaseRequest) (*PhaseResponse, error)
}

// PhaseRequest is the input for PhaseBackend.Execute.
type PhaseRequest struct {
	Prompt  string
	PhaseID PhaseID
}

// PhaseResponse is the output from PhaseBackend.Execute.
type PhaseResponse struct {
	Output string
}

// EngineConfig is the configuration for SubprocessEngine.
type EngineConfig struct {
	SpecID     string
	Platform   string
	Strategy   Strategy
	Backend    PhaseBackend
	Checkpoint *Checkpoint
	DryRun     bool
}

// PipelineResult holds the outcome of a pipeline run.
type PipelineResult struct {
	PhaseResults []PhaseResult
}

// PhaseResult holds the outcome of a single phase execution.
type PhaseResult struct {
	PhaseID PhaseID
	Output  string
	Verdict GateVerdict
}

// noopBackend is the default backend used when none is configured.
// It returns empty responses without calling any subprocess.
type noopBackend struct{}

func (n *noopBackend) Execute(_ context.Context, _ PhaseRequest) (*PhaseResponse, error) {
	return &PhaseResponse{}, nil
}

// SubprocessEngine implements PipelineEngine using subprocess execution.
type SubprocessEngine struct {
	cfg EngineConfig
}

// @AX:ANCHOR: [AUTO] public API contract — entry point called from CLI and tests (fan-in >= 3)
// NewSubprocessEngine creates a SubprocessEngine with the given config.
func NewSubprocessEngine(cfg EngineConfig) *SubprocessEngine {
	if cfg.Backend == nil {
		cfg.Backend = &noopBackend{}
	}
	return &SubprocessEngine{cfg: cfg}
}

// @AX:ANCHOR: [AUTO] architectural boundary — sole orchestration entry point for 5-phase pipeline
// Run executes the full 5-phase pipeline.
func (e *SubprocessEngine) Run(ctx context.Context) (*PipelineResult, error) {
	phases := DefaultPhases()

	results := make([]PhaseResult, len(phases))
	var previousOutput string

	for i, phase := range phases {
		phaseID := phase.ID

		// Pre-populate result with phase ID so skipped phases still appear.
		results[i] = PhaseResult{PhaseID: phaseID}

		// Skip phases that are already done in the checkpoint.
		if e.cfg.Checkpoint != nil {
			status, found := e.cfg.Checkpoint.TaskStatus[string(phaseID)]
			if found && status == CheckpointStatusDone {
				continue
			}
		}

		// Build prompt by injecting previous phase output.
		prompt := buildPrompt(e.cfg.SpecID, phaseID, previousOutput)

		// In dry-run mode, generate the prompt but do not invoke the backend.
		if e.cfg.DryRun {
			results[i] = PhaseResult{PhaseID: phaseID, Output: prompt}
			continue
		}

		req := PhaseRequest{
			Prompt:  prompt,
			PhaseID: phaseID,
		}

		resp, err := e.cfg.Backend.Execute(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("phase %s: %w", phaseID, err)
		}

		results[i] = PhaseResult{
			PhaseID: phaseID,
			Output:  resp.Output,
		}
		previousOutput = resp.Output
	}

	return &PipelineResult{PhaseResults: results}, nil
}

// @AX:NOTE: [AUTO] magic constant in format string — SPEC/Phase labels are part of prompt contract
// buildPrompt assembles the prompt for a phase, injecting prior output when available.
func buildPrompt(specID string, phaseID PhaseID, previousOutput string) string {
	prompt := fmt.Sprintf("SPEC: %s\nPhase: %s", specID, phaseID)
	if previousOutput != "" {
		prompt += fmt.Sprintf("\n\nPrevious phase output:\n%s", previousOutput)
	}
	return prompt
}
