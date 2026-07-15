package worker

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/compress"
	"github.com/insajin/autopus-adk/pkg/worker/controlplane"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
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
	Usage      []telemetry.UsageEnvelope
}

// @AX:NOTE: [AUTO] hardcoded phase order defines the worker phase-split default; keep aligned with prompts and server phase plans
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
	compressor           compress.ContextCompressor
	compactionRecorder   func(compress.CompactionEvent)
	router               *routing.Router // nil if routing not configured
	interruptRecorder    func(AuditEvent)
	runID                string
	attempt              int
	effort               string
	role                 string
	usageProvenance      usageProvenance
	requiredContext      string
	persistentTask       string
}

// NewPipelineExecutor creates a new PipelineExecutor.
func NewPipelineExecutor(provider adapter.ProviderAdapter, mcpConfig, workDir string) *PipelineExecutor {
	return &PipelineExecutor{
		provider:   provider,
		mcpConfig:  mcpConfig,
		workDir:    workDir,
		compressor: compress.NewDefaultCompressor(2),
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

// SetCompactionRecorder configures a typed sink for applied compaction and
// fail-closed integrity events.
func (pe *PipelineExecutor) SetCompactionRecorder(record func(compress.CompactionEvent)) {
	pe.compactionRecorder = record
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

// SetInterruptRecorder configures structured interrupt evidence recording.
func (pe *PipelineExecutor) SetInterruptRecorder(record func(AuditEvent)) {
	pe.interruptRecorder = record
}

// SetRouter configures model routing for the pipeline (REQ-ROUTE-01).
func (pe *PipelineExecutor) SetRouter(r *routing.Router) {
	pe.router = r
}

// SetUsageIdentity configures task-wide usage identity. Each phase still gets
// its own call identity while sharing this run identity.
func (pe *PipelineExecutor) SetUsageIdentity(runID string, attempt int, effort, role string) {
	pe.runID = runID
	pe.attempt = attempt
	pe.effort = effort
	pe.role = role
}

// SetUsageProvenance configures supervisor-trusted usage provenance for every phase.
func (pe *PipelineExecutor) SetUsageProvenance(providerVersion, modelVersion, riskPolicy, cacheStratum, configHash string) {
	pe.usageProvenance = usageProvenance{providerVersion, modelVersion, riskPolicy, cacheStratum, configHash}
}

// Execute runs the full pipeline: planner -> executor(s) -> tester -> reviewer.
// Each phase uses an independent --resume session ID.
// Returns an aggregated TaskResult combining all phase outputs.
func (pe *PipelineExecutor) Execute(ctx context.Context, taskID, prompt string) (adapter.TaskResult, error) {
	return pe.ExecuteWithPlan(ctx, taskID, prompt, "", nil)
}

// @AX:ANCHOR: [AUTO] public phase-split execution contract called by Execute, worker loop, and integration tests (fan-in >= 3)
// @AX:REASON: Signature and phase/blocker semantics coordinate subprocess execution, routing, compression, and budget accounting.
// ExecuteWithPlan runs the pipeline with an optional server-selected model and
// explicit phase plan. When phases is empty, the default sequence is used.
func (pe *PipelineExecutor) ExecuteWithPlan(ctx context.Context, taskID, prompt, model string, phases []Phase) (adapter.TaskResult, error) {
	log.Printf("[pipeline] starting phase-split for task %s", taskID)

	routedModel := pe.resolveModel(model, prompt)
	identity := ensureUsageIdentity(adapter.TaskConfig{
		TaskID: taskID, RunID: pe.runID, Attempt: pe.attempt, Effort: pe.effort, Role: pe.role,
	}, "pipeline", "worker")
	pe.runID, pe.attempt = identity.RunID, identity.Attempt
	prevOutput := prompt
	normalizedPhases := normalizePhasePlan(phases)
	if len(normalizedPhases) == 0 {
		return adapter.TaskResult{}, fmt.Errorf("missing pipeline phase plan")
	}

	var (
		results       []PhaseResult
		totalCost     float64
		totalDuration int64
	)
	for phaseIndex, phase := range normalizedPhases {
		select {
		case <-ctx.Done():
			return pe.aggregateResults(results, totalCost, totalDuration), ctx.Err()
		default:
		}

		pe.logPhaseBudget(phase)

		phasePrompt, err := pe.phasePrompt(phase, prevOutput)
		if err != nil {
			return pe.aggregateResults(results, totalCost, totalDuration), err
		}
		result, err := pe.runPhaseWithTaskContext(ctx, taskID, phase, phasePrompt, routedModel, phaseIndex > 0)
		if err != nil {
			log.Printf("[pipeline] phase %s failed for task %s: %v", phase, taskID, err)
			results = append(results, result)
			totalCost += result.CostUSD
			totalDuration += result.DurationMS
			return pe.aggregateResults(results, totalCost, totalDuration), fmt.Errorf("phase %s: %w", phase, err)
		}

		pe.completePhase(phase, result.ToolCalls)

		results = append(results, result)
		totalCost += result.CostUSD
		totalDuration += result.DurationMS
		nextOutput, err := pe.nextPhaseInput(phase, result.Output)
		if err != nil {
			return pe.aggregateResults(results, totalCost, totalDuration), err
		}
		prevOutput = nextOutput

		log.Printf("[pipeline] phase %s completed: cost=$%.4f duration=%dms", phase, result.CostUSD, result.DurationMS)
	}

	return pe.aggregateResults(results, totalCost, totalDuration), nil
}

func (pe *PipelineExecutor) resolveModel(model, prompt string) string {
	if model != "" || pe.router == nil || controlplane.SignedControlPlaneEnforced() {
		return model
	}
	return pe.router.Route(pe.provider.Name(), prompt)
}

func (pe *PipelineExecutor) logPhaseBudget(phase Phase) {
	if pe.allocator == nil {
		return
	}
	log.Printf("[pipeline] phase %s budget: %d tool calls", phase, pe.allocator.PhaseLimit(string(phase)))
}

func (pe *PipelineExecutor) completePhase(phase Phase, toolCalls int) {
	if pe.allocator == nil {
		return
	}
	pe.allocator.CompletePhase(string(phase), toolCalls)
	log.Printf("[pipeline] phase %s used %d tool calls, remaining total: %d",
		phase, toolCalls, pe.allocator.TotalRemaining())
}

// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: compaction blockers fail closed before building the next phase prompt
func (pe *PipelineExecutor) nextPhaseInput(phase Phase, output string) (string, error) {
	if pe.compressor == nil {
		return output, nil
	}
	if detailed, ok := pe.compressor.(interface {
		CompressDetailed(string, string, string) compress.CompactionResult
	}); ok {
		result := detailed.CompressDetailed(string(phase), output, pe.provider.Name())
		if pe.compactionRecorder != nil && (result.Event.CompactionApplied || result.Blocker != "") {
			pe.compactionRecorder(result.Event)
		}
		if result.Blocker != "" {
			return "", fmt.Errorf("phase %s compaction blocker: %s", phase, result.Blocker)
		}
		if result.Event.CompactionApplied {
			log.Printf("[pipeline] compaction event phase=%s summary_id=%s input=%d output=%d pruned_pairs=%d reasons=%s",
				phase,
				result.Event.SummaryID,
				result.Event.InputEstimate,
				result.Event.OutputEstimate,
				result.Event.PrunedPairCount,
				strings.Join(result.Event.ReasonCodes, ","),
			)
		}
		return result.Output, nil
	}
	return pe.compressor.Compress(string(phase), output, pe.provider.Name()), nil
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
