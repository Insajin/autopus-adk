// Package pipeline provides pipeline state management types and persistence.
package pipeline

import (
	"context"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/worker/compress"
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
	Attempt int
}

// PhaseResponse is the output from PhaseBackend.Execute.
type PhaseResponse struct {
	Output       string
	Provider     string
	Backend      string
	Role         string
	ExitCode     int
	TimedOut     bool
	FailureClass string
	Artifact     string
}

// EngineConfig is the configuration for SubprocessEngine.
type EngineConfig struct {
	SpecID string
	// SpecDir is the resolved, trusted directory containing required SPEC documents.
	SpecDir    string
	Platform   string
	Strategy   Strategy
	Backend    PhaseBackend
	Checkpoint *Checkpoint
	DryRun     bool
	// SnapshotHash binds resume state to the resolved SPEC body.
	SnapshotHash string
	// GitCommitHash binds persisted state to the current source revision.
	GitCommitHash string
	// Compressor compacts previous phase output before injecting it into the next prompt.
	Compressor compress.ContextCompressor
	// RunConfig holds runner-level configuration including the learn store.
	RunConfig RunConfig
}

// PipelineResult holds the outcome of a pipeline run.
type PipelineResult struct {
	PhaseResults     []PhaseResult
	CompactionEvents []compress.CompactionEvent
	Receipt          OrchestrationRunReceipt
}

// PhaseResult holds the outcome of a single phase execution.
type PhaseResult struct {
	PhaseID         PhaseID
	Output          string
	Verdict         GateVerdict
	Status          CheckpointStatus
	Attempts        int
	CompactionEvent *compress.CompactionEvent
}

// SubprocessEngine implements PipelineEngine using subprocess execution.
type SubprocessEngine struct {
	cfg           EngineConfig
	promptBuilder *PhasePromptBuilder
}

// @AX:ANCHOR: [AUTO] @AX:REASON: public API contract — entry point called from CLI and tests (fan-in >= 3)
// NewSubprocessEngine creates a SubprocessEngine with the given config.
func NewSubprocessEngine(cfg EngineConfig) *SubprocessEngine {
	if cfg.Compressor == nil {
		// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: keepRecent=2 is the default phase-transition compaction policy
		cfg.Compressor = compress.NewDefaultCompressor(2)
	}
	engine := &SubprocessEngine{cfg: cfg}
	if cfg.SpecDir != "" {
		engine.promptBuilder = NewPhasePromptBuilder(cfg.SpecDir)
	}
	return engine
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

// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: compaction blockers abort before the next backend call to avoid lossy prompt handoff
func (e *SubprocessEngine) compactPhaseOutput(phaseID PhaseID, output string) (string, *compress.CompactionEvent, error) {
	if e.cfg.Compressor == nil {
		return output, nil, nil
	}
	if detailed, ok := e.cfg.Compressor.(interface {
		CompressDetailed(string, string, string) compress.CompactionResult
	}); ok {
		result := detailed.CompressDetailed(string(phaseID), output, e.cfg.Platform)
		if result.Blocker != "" {
			return "", &result.Event, fmt.Errorf("phase %s compaction blocker: %s", phaseID, result.Blocker)
		}
		if !result.Event.CompactionApplied {
			return result.Output, nil, nil
		}
		return result.Output, &result.Event, nil
	}
	compressed := e.cfg.Compressor.Compress(string(phaseID), output, e.cfg.Platform)
	if compressed == output {
		return output, nil, nil
	}
	return compressed, nil, nil
}
