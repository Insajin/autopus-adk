package worker

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/compress"
	"github.com/insajin/autopus-adk/pkg/worker/stream"
)

// Phase represents a pipeline execution phase.
type Phase string

const (
	PhasePlanner  Phase = "planner"
	PhaseExecutor Phase = "executor"
	PhaseTester   Phase = "tester"
	PhaseReviewer Phase = "reviewer"
)

// PhaseResult holds the output from a single pipeline phase.
type PhaseResult struct {
	Phase      Phase
	Output     string
	CostUSD    float64
	DurationMS int64
	SessionID  string
	ToolCalls  int // number of tool calls made during this phase
}

// PipelineExecutor spawns separate subprocesses for each phase:
// planner -> executor(s) -> tester -> reviewer.
// Triggered when a single --print execution exceeds the context window.
type PipelineExecutor struct {
	provider   adapter.ProviderAdapter
	mcpConfig  string
	workDir    string
	allocator  *budget.PhaseAllocator    // nil if budget not configured
	compressor compress.ContextCompressor // nil if compression not configured
}

// NewPipelineExecutor creates a new PipelineExecutor.
func NewPipelineExecutor(provider adapter.ProviderAdapter, mcpConfig, workDir string) *PipelineExecutor {
	return &PipelineExecutor{
		provider:  provider,
		mcpConfig: mcpConfig,
		workDir:   workDir,
	}
}

// SetBudget configures per-phase budget allocation for the pipeline.
func (pe *PipelineExecutor) SetBudget(total int, alloc budget.PhaseAllocation) {
	pe.allocator = budget.NewPhaseAllocator(total, alloc)
}

// SetCompressor configures context compression for phase transitions.
func (pe *PipelineExecutor) SetCompressor(c compress.ContextCompressor) {
	pe.compressor = c
}

// Execute runs the full pipeline: planner → executor(s) → tester → reviewer.
// Each phase uses an independent --resume session ID.
// Returns an aggregated TaskResult combining all phase outputs.
func (pe *PipelineExecutor) Execute(ctx context.Context, taskID, prompt string) (adapter.TaskResult, error) {
	log.Printf("[pipeline] starting phase-split for task %s", taskID)

	phases := []struct {
		phase      Phase
		promptFunc func(string) string
	}{
		{PhasePlanner, pe.plannerPrompt},
		{PhaseExecutor, pe.executorPrompt},
		{PhaseTester, pe.testerPrompt},
		{PhaseReviewer, pe.reviewerPrompt},
	}

	var results []PhaseResult
	var totalCost float64
	var totalDuration int64
	prevOutput := prompt

	for _, p := range phases {
		select {
		case <-ctx.Done():
			return adapter.TaskResult{}, ctx.Err()
		default:
		}

		// Log phase budget if allocator is configured (REQ-BUDGET-09).
		if pe.allocator != nil {
			limit := pe.allocator.PhaseLimit(string(p.phase))
			log.Printf("[pipeline] phase %s budget: %d tool calls", p.phase, limit)
		}

		phasePrompt := p.promptFunc(prevOutput)
		pr, err := pe.runPhase(ctx, taskID, p.phase, phasePrompt)
		if err != nil {
			log.Printf("[pipeline] phase %s failed for task %s: %v", p.phase, taskID, err)
			return adapter.TaskResult{}, fmt.Errorf("phase %s: %w", p.phase, err)
		}

		// Record phase completion for budget carry-over (REQ-BUDGET-10).
		if pe.allocator != nil {
			pe.allocator.CompletePhase(string(p.phase), pr.ToolCalls)
			log.Printf("[pipeline] phase %s used %d tool calls, remaining total: %d",
				p.phase, pr.ToolCalls, pe.allocator.TotalRemaining())
		}

		results = append(results, pr)
		totalCost += pr.CostUSD
		totalDuration += pr.DurationMS

		// Compress phase output before passing to next phase (REQ-COMP-001).
		if pe.compressor != nil {
			prevOutput = pe.compressor.Compress(string(p.phase), pr.Output, pe.provider.Name())
		} else {
			prevOutput = pr.Output
		}

		log.Printf("[pipeline] phase %s completed: cost=$%.4f duration=%dms", p.phase, pr.CostUSD, pr.DurationMS)
	}

	return pe.aggregateResults(results, totalCost, totalDuration), nil
}

// runPhase spawns a single subprocess for the given phase.
func (pe *PipelineExecutor) runPhase(ctx context.Context, taskID string, phase Phase, prompt string) (PhaseResult, error) {
	sessionID := fmt.Sprintf("pipeline-%s-%s", taskID, phase)
	taskCfg := adapter.TaskConfig{
		TaskID:    fmt.Sprintf("%s-%s", taskID, phase),
		SessionID: sessionID,
		Prompt:    prompt,
		MCPConfig: pe.mcpConfig,
		WorkDir:   pe.workDir,
	}

	cmd := pe.provider.BuildCommand(ctx, taskCfg)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return PhaseResult{}, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PhaseResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return PhaseResult{}, fmt.Errorf("start subprocess: %w", err)
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, strings.NewReader(prompt))
	}()

	result, parseErr := pe.parsePhaseStream(stdout, phase)

	waitErr := cmd.Wait()
	if parseErr != nil {
		return PhaseResult{}, fmt.Errorf("parse stream: %w", parseErr)
	}
	if waitErr != nil {
		if result.Output != "" {
			return result, nil
		}
		return PhaseResult{}, fmt.Errorf("subprocess exit: %w", waitErr)
	}

	return result, nil
}

// parsePhaseStream reads subprocess stdout and extracts the phase result.
// Counts tool_call and tool_use events for budget tracking.
func (pe *PipelineExecutor) parsePhaseStream(r io.Reader, phase Phase) (PhaseResult, error) {
	parser := stream.NewParser(r)
	var result PhaseResult
	result.Phase = phase
	hasResult := false

	for {
		evt, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return PhaseResult{}, err
		}

		// Count tool calls for budget tracking (REQ-BUDGET-05).
		if evt.Type == stream.EventToolCall || evt.Type == "tool_use" {
			result.ToolCalls++
		}

		if evt.Type == "result" {
			adapterEvt := adapter.StreamEvent{Type: evt.Type, Subtype: evt.Subtype, Data: evt.Raw}
			tr := pe.provider.ExtractResult(adapterEvt)
			result.Output = tr.Output
			result.CostUSD = tr.CostUSD
			result.DurationMS = tr.DurationMS
			result.SessionID = tr.SessionID
			hasResult = true
		}
	}

	if !hasResult {
		return PhaseResult{}, fmt.Errorf("no result event for phase %s", phase)
	}
	return result, nil
}

// aggregateResults combines all phase results into a single TaskResult.
func (pe *PipelineExecutor) aggregateResults(results []PhaseResult, totalCost float64, totalDuration int64) adapter.TaskResult {
	var sb strings.Builder
	for _, r := range results {
		fmt.Fprintf(&sb, "## Phase: %s\n\n%s\n\n", r.Phase, r.Output)
	}
	return adapter.TaskResult{
		CostUSD:    totalCost,
		DurationMS: totalDuration,
		Output:     sb.String(),
	}
}

// Phase-specific prompt wrappers inject role context for each phase.

func (pe *PipelineExecutor) plannerPrompt(input string) string {
	return fmt.Sprintf("You are the PLANNER phase. Analyze the task and create an execution plan.\n\n%s", input)
}

func (pe *PipelineExecutor) executorPrompt(plannerOutput string) string {
	return fmt.Sprintf("You are the EXECUTOR phase. Implement the plan below.\n\n%s", plannerOutput)
}

func (pe *PipelineExecutor) testerPrompt(executorOutput string) string {
	return fmt.Sprintf("You are the TESTER phase. Write and run tests for the implementation below.\n\n%s", executorOutput)
}

func (pe *PipelineExecutor) reviewerPrompt(testerOutput string) string {
	return fmt.Sprintf("You are the REVIEWER phase. Review the implementation and test results below.\n\n%s", testerOutput)
}

// IsContextOverflow checks whether a stream event indicates a context window overflow.
// Returns true if the event is an error containing "context window" or "token limit".
func IsContextOverflow(evt adapter.StreamEvent) bool {
	if evt.Type != "error" {
		return false
	}
	lower := strings.ToLower(string(evt.Data))
	return strings.Contains(lower, "context window") || strings.Contains(lower, "token limit")
}
