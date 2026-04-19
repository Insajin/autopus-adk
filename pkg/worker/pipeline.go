package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/compress"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
	"github.com/insajin/autopus-adk/pkg/worker/security"
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

var defaultPipelinePhases = []Phase{
	PhasePlanner,
	PhaseExecutor,
	PhaseTester,
	PhaseReviewer,
}

// PipelineExecutor spawns separate subprocesses for each phase:
// planner -> executor(s) -> tester -> reviewer.
// Triggered when a single --print execution exceeds the context window.
type PipelineExecutor struct {
	provider             adapter.ProviderAdapter
	mcpConfig            string
	workDir              string
	envVars              map[string]string
	phaseInstructions    map[Phase]string
	phasePromptTemplates map[Phase]string
	allocator            *budget.PhaseAllocator // nil if budget not configured
	iterationBudget      *budget.IterationBudget
	compressor           compress.ContextCompressor // nil if compression not configured
	router               *routing.Router            // nil if routing not configured
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

// SetIterationBudget configures a server-issued total iteration budget for the pipeline.
func (pe *PipelineExecutor) SetIterationBudget(iterationBudget budget.IterationBudget) {
	pe.iterationBudget = &iterationBudget
	pe.allocator = budget.NewPhaseAllocator(iterationBudget.Limit, budget.DefaultAllocation())
}

// SetCompressor configures context compression for phase transitions.
func (pe *PipelineExecutor) SetCompressor(c compress.ContextCompressor) {
	pe.compressor = c
}

// SetEnvVars configures additional environment variables for all pipeline phases.
func (pe *PipelineExecutor) SetEnvVars(envVars map[string]string) {
	if len(envVars) == 0 {
		pe.envVars = nil
		return
	}
	pe.envVars = make(map[string]string, len(envVars))
	for k, v := range envVars {
		pe.envVars[k] = v
	}
}

// SetPhaseInstructions configures server-selected instructions for pipeline phases.
func (pe *PipelineExecutor) SetPhaseInstructions(instructions map[Phase]string) {
	if len(instructions) == 0 {
		pe.phaseInstructions = nil
		return
	}
	pe.phaseInstructions = make(map[Phase]string, len(instructions))
	for phase, instruction := range instructions {
		pe.phaseInstructions[phase] = instruction
	}
}

// SetPhasePromptTemplates configures server-selected full prompt templates for pipeline phases.
func (pe *PipelineExecutor) SetPhasePromptTemplates(templates map[Phase]string) {
	if len(templates) == 0 {
		pe.phasePromptTemplates = nil
		return
	}
	pe.phasePromptTemplates = make(map[Phase]string, len(templates))
	for phase, template := range templates {
		pe.phasePromptTemplates[phase] = template
	}
}

// SetRouter configures model routing for the pipeline (REQ-ROUTE-01).
func (pe *PipelineExecutor) SetRouter(r *routing.Router) {
	pe.router = r
}

// Execute runs the full pipeline: planner → executor(s) → tester → reviewer.
// Each phase uses an independent --resume session ID.
// Returns an aggregated TaskResult combining all phase outputs.
func (pe *PipelineExecutor) Execute(ctx context.Context, taskID, prompt string) (adapter.TaskResult, error) {
	return pe.ExecuteWithPlan(ctx, taskID, prompt, "", nil)
}

// ExecuteWithPlan runs the pipeline with an optional server-selected model and
// explicit phase plan. When phases is empty, the default sequence is used.
func (pe *PipelineExecutor) ExecuteWithPlan(ctx context.Context, taskID, prompt, model string, phases []Phase) (adapter.TaskResult, error) {
	log.Printf("[pipeline] starting phase-split for task %s", taskID)

	// Resolve model once from the original prompt (REQ-ROUTE-01).
	// In signed control-plane mode, local routing fallback is disabled.
	routedModel := model
	if routedModel == "" && pe.router != nil && !a2a.SignedControlPlaneEnforced() {
		routedModel = pe.router.Route(pe.provider.Name(), prompt)
	}

	var results []PhaseResult
	var totalCost float64
	var totalDuration int64
	prevOutput := prompt
	normalizedPhases := normalizePhasePlan(phases)
	if len(normalizedPhases) == 0 {
		return adapter.TaskResult{}, fmt.Errorf("missing pipeline phase plan")
	}

	for _, phase := range normalizedPhases {
		select {
		case <-ctx.Done():
			return adapter.TaskResult{}, ctx.Err()
		default:
		}

		// Log phase budget if allocator is configured (REQ-BUDGET-09).
		if pe.allocator != nil {
			limit := pe.allocator.PhaseLimit(string(phase))
			log.Printf("[pipeline] phase %s budget: %d tool calls", phase, limit)
		}

		phasePrompt, err := pe.phasePrompt(phase, prevOutput)
		if err != nil {
			return adapter.TaskResult{}, err
		}
		pr, err := pe.runPhase(ctx, taskID, phase, phasePrompt, routedModel)
		if err != nil {
			log.Printf("[pipeline] phase %s failed for task %s: %v", phase, taskID, err)
			return adapter.TaskResult{}, fmt.Errorf("phase %s: %w", phase, err)
		}

		// Record phase completion for budget carry-over (REQ-BUDGET-10).
		if pe.allocator != nil {
			pe.allocator.CompletePhase(string(phase), pr.ToolCalls)
			log.Printf("[pipeline] phase %s used %d tool calls, remaining total: %d",
				phase, pr.ToolCalls, pe.allocator.TotalRemaining())
		}

		results = append(results, pr)
		totalCost += pr.CostUSD
		totalDuration += pr.DurationMS

		// Compress phase output before passing to next phase (REQ-COMP-001).
		if pe.compressor != nil {
			prevOutput = pe.compressor.Compress(string(phase), pr.Output, pe.provider.Name())
		} else {
			prevOutput = pr.Output
		}

		log.Printf("[pipeline] phase %s completed: cost=$%.4f duration=%dms", phase, pr.CostUSD, pr.DurationMS)
	}

	return pe.aggregateResults(results, totalCost, totalDuration), nil
}

// ParsePhasePlan validates and canonicalizes a server-provided phase plan.
func ParsePhasePlan(phases []string) ([]Phase, error) {
	if len(phases) == 0 {
		return nil, nil
	}

	plan := make([]Phase, 0, len(phases))
	for _, raw := range phases {
		phase, err := ParsePhase(raw)
		if err != nil {
			return nil, err
		}
		plan = append(plan, phase)
	}
	if len(plan) == 0 {
		return nil, nil
	}
	return plan, nil
}

// ParsePhase validates and canonicalizes a single phase name.
func ParsePhase(name string) (Phase, error) {
	switch phase := Phase(strings.ToLower(strings.TrimSpace(name))); phase {
	case PhasePlanner, PhaseExecutor, PhaseTester, PhaseReviewer:
		return phase, nil
	case "":
		return "", fmt.Errorf("empty phase name")
	default:
		return "", fmt.Errorf("unsupported phase %q", name)
	}
}

// ParsePhaseInstructions validates phase instruction overrides from the server.
func ParsePhaseInstructions(instructions map[string]string) (map[Phase]string, error) {
	if len(instructions) == 0 {
		return nil, nil
	}

	parsed := make(map[Phase]string, len(instructions))
	for rawPhase, instruction := range instructions {
		phase, err := ParsePhase(rawPhase)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(instruction) == "" {
			continue
		}
		parsed[phase] = strings.TrimSpace(instruction)
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	return parsed, nil
}

// ParsePhasePromptTemplates validates server-provided full prompt templates.
func ParsePhasePromptTemplates(templates map[string]string) (map[Phase]string, error) {
	if len(templates) == 0 {
		return nil, nil
	}

	parsed := make(map[Phase]string, len(templates))
	for rawPhase, template := range templates {
		phase, err := ParsePhase(rawPhase)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(template) == "" {
			continue
		}
		parsed[phase] = strings.TrimSpace(template)
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	return parsed, nil
}

func normalizePhasePlan(phases []Phase) []Phase {
	if len(phases) == 0 && a2a.SignedControlPlaneEnforced() {
		return nil
	}
	if len(phases) == 0 {
		return append([]Phase(nil), defaultPipelinePhases...)
	}
	return append([]Phase(nil), phases...)
}

func (pe *PipelineExecutor) phasePrompt(phase Phase, input string) (string, error) {
	if template, ok := pe.phasePromptTemplates[phase]; ok && strings.TrimSpace(template) != "" {
		return renderPhasePromptTemplate(template, input), nil
	}
	if instruction, ok := pe.phaseInstructions[phase]; ok && strings.TrimSpace(instruction) != "" {
		return fmt.Sprintf("%s\n\n%s", instruction, input), nil
	}
	if a2a.SignedControlPlaneEnforced() {
		return input, nil
	}

	switch phase {
	case PhasePlanner:
		return pe.plannerPrompt(input), nil
	case PhaseExecutor:
		return pe.executorPrompt(input), nil
	case PhaseTester:
		return pe.testerPrompt(input), nil
	case PhaseReviewer:
		return pe.reviewerPrompt(input), nil
	default:
		return "", fmt.Errorf("unsupported phase %q", phase)
	}
}

func renderPhasePromptTemplate(template, input string) string {
	if strings.Contains(template, "{{input}}") {
		return strings.ReplaceAll(template, "{{input}}", input)
	}
	return fmt.Sprintf("%s\n\n%s", template, input)
}

// runPhase spawns a single subprocess for the given phase.
func (pe *PipelineExecutor) runPhase(ctx context.Context, taskID string, phase Phase, prompt, model string) (PhaseResult, error) {
	sessionID := fmt.Sprintf("pipeline-%s-%s", taskID, phase)
	taskCfg := adapter.TaskConfig{
		TaskID:    fmt.Sprintf("%s-%s", taskID, phase),
		SessionID: sessionID,
		Prompt:    prompt,
		MCPConfig: pe.mcpConfig,
		WorkDir:   pe.workDir,
		Model:     model,
	}
	if len(pe.envVars) > 0 {
		taskCfg.EnvVars = make(map[string]string, len(pe.envVars))
		for k, v := range pe.envVars {
			taskCfg.EnvVars[k] = v
		}
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

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return PhaseResult{}, fmt.Errorf("start subprocess: %w", err)
	}

	var emergencyStop *security.EmergencyStop
	phaseBudget := pe.phaseIterationBudget(phase)
	if phaseBudget != nil {
		emergencyStop = security.NewEmergencyStop()
		emergencyStop.SetProcess(cmd)
		defer emergencyStop.ClearProcess()
	}

	go func() {
		defer func() {
			if err := stdin.Close(); err != nil {
				log.Printf("[pipeline] stdin close failed for %s: %v", phase, err)
			}
		}()
		if _, err := io.Copy(stdin, strings.NewReader(prompt)); err != nil {
			log.Printf("[pipeline] prompt write failed for %s: %v", phase, err)
		}
	}()

	result, parseErr := pe.parsePhaseStream(stdout, phase, phaseBudget, emergencyStop)

	waitErr := cmd.Wait()
	if parseErr != nil {
		if stderrStr := strings.TrimSpace(stderrBuf.String()); stderrStr != "" {
			return PhaseResult{}, fmt.Errorf("parse stream: %w\nstderr: %s", parseErr, stderrStr)
		}
		return PhaseResult{}, fmt.Errorf("parse stream: %w", parseErr)
	}
	if waitErr != nil {
		if result.Output != "" {
			return result, nil
		}
		if stderrStr := strings.TrimSpace(stderrBuf.String()); stderrStr != "" {
			return PhaseResult{}, fmt.Errorf("subprocess exit: %w\nstderr: %s", waitErr, stderrStr)
		}
		return PhaseResult{}, fmt.Errorf("subprocess exit: %w", waitErr)
	}

	return result, nil
}

// parsePhaseStream reads subprocess stdout and extracts the phase result.
// Counts tool_call and tool_use events for budget tracking.
func (pe *PipelineExecutor) parsePhaseStream(r io.Reader, phase Phase, phaseBudget *budget.IterationBudget, emergencyStop *security.EmergencyStop) (PhaseResult, error) {
	scanner := bufio.NewScanner(r)
	var result PhaseResult
	result.Phase = phase
	hasResult := false
	var counter *budget.Counter
	if phaseBudget != nil {
		counter = budget.NewCounter(*phaseBudget)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		evt, err := pe.provider.ParseEvent([]byte(line))
		if err != nil {
			log.Printf("[stream] skipping malformed line: %v", err)
			continue
		}

		// Count tool calls for budget tracking (REQ-BUDGET-05).
		if evt.Type == stream.EventToolCall || evt.Type == "tool_use" {
			result.ToolCalls++
			if counter != nil {
				state := counter.Increment()
				if state.Level == budget.LevelExhausted && emergencyStop != nil {
					log.Printf("[pipeline] phase %s budget exhausted, stopping", phase)
					_ = emergencyStop.Stop("pipeline_iteration_budget_exceeded")
					return result, fmt.Errorf("phase %s iteration budget exceeded: %d/%d", phase, state.Count, state.Budget.Limit)
				}
			}
		}

		if evt.Type == "result" {
			tr := pe.provider.ExtractResult(evt)
			result.Output = tr.Output
			result.CostUSD = tr.CostUSD
			result.DurationMS = tr.DurationMS
			result.SessionID = tr.SessionID
			hasResult = true
		}
	}
	if err := scanner.Err(); err != nil {
		return PhaseResult{}, fmt.Errorf("stream scan: %w", err)
	}

	if !hasResult {
		return PhaseResult{}, fmt.Errorf("no result event for phase %s", phase)
	}
	return result, nil
}

func (pe *PipelineExecutor) phaseIterationBudget(phase Phase) *budget.IterationBudget {
	if pe.iterationBudget == nil || pe.allocator == nil {
		return nil
	}
	phaseBudget := *pe.iterationBudget
	phaseBudget.Limit = pe.allocator.PhaseLimit(string(phase))
	if phaseBudget.Limit <= 0 {
		return nil
	}
	return &phaseBudget
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
