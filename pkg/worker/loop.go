package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/insajin/autopus-adk/pkg/worker/auth"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	workerNet "github.com/insajin/autopus-adk/pkg/worker/net"
	"github.com/insajin/autopus-adk/pkg/worker/parallel"
	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
	"github.com/insajin/autopus-adk/pkg/worker/reaper"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
	"github.com/insajin/autopus-adk/pkg/worker/scheduler"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
	"github.com/insajin/autopus-adk/pkg/worker/tui"
)

// LoopConfig holds configuration for the WorkerLoop.
type LoopConfig struct {
	BackendURL    string
	WorkerName    string
	MemoryAgentID string
	Skills        []string
	Provider      adapter.ProviderAdapter
	MCPConfig     string          // path to worker-mcp.json
	WorkDir       string          // working directory for subprocesses
	AuthToken     string          // bearer token for backend auth
	Router        *routing.Router // optional model router (nil = no routing)
	// Deprecated: use CredentialStore instead. Kept for backward compatibility.
	CredentialsPath   string                // path to credentials.json for token refresh
	CredentialStore   setup.CredentialStore // Secure credential storage (Keychain/encrypted file). If nil and CredentialsPath is set, falls back to plain file mode.
	AuditLogPath      string                // audit log file path (default: {WorkDir}/.autopus/audit.jsonl)
	AuditMaxSize      int64                 // max log size before rotation (default: 10MB)
	AuditMaxAge       time.Duration         // max age of rotated files (default: 7 days)
	WorkspaceID       string                // workspace identifier for scheduler
	MaxConcurrency    int                   // max parallel tasks (0 or 1 = sequential)
	WorktreeIsolation bool                  // enable worktree isolation for parallel tasks
	KnowledgeSync     bool                  // enable local knowledge context loading
	KnowledgeDir      string                // local knowledge directory hint (defaults to WorkDir)
}

// WorkerLoop integrates A2A Server, ProviderAdapter, ContextBuilder, and StreamParser.
// It receives tasks via A2A, builds prompts, spawns CLI subprocesses, and reports results.
type WorkerLoop struct {
	config            LoopConfig
	server            *a2a.Server
	builder           ContextBuilder
	tuiProgram        *tea.Program
	authRefresher     *auth.TokenRefresher
	authReconnector   *auth.Reconnector
	netMonitor        *workerNet.NetMonitor
	lifecycleCtx      context.Context
	lifecycleCancel   context.CancelFunc
	auditWriter       *audit.RotatingWriter
	knowledgeSearcher *knowledge.KnowledgeSearcher
	memorySearcher    *knowledge.MemorySearcher
	schedulerDisp     *scheduler.Dispatcher
	semaphore         *parallel.TaskSemaphore
	worktreeManager   *parallel.WorktreeManager
	auditLogger       *slogAuditLogger
	pidLock           *pidlock.Lock
	zombieReaper      *reaper.Reaper
}

// NewWorkerLoop creates a WorkerLoop with the given configuration.
func NewWorkerLoop(config LoopConfig) *WorkerLoop {
	wl := &WorkerLoop{
		config:      config,
		auditLogger: newSlogAuditLogger(3),
	}

	serverCfg := a2a.ServerConfig{
		BackendURL:            config.BackendURL,
		WorkerName:            config.WorkerName,
		WorkspaceID:           config.WorkspaceID,
		Skills:                config.Skills,
		Handler:               wl.handleTask,
		AuthToken:             config.AuthToken,
		ApprovalCallback:      wl.handleApproval,
		OnConnectionExhausted: wl.activateFallbackPoller,
	}
	wl.server = a2a.NewServer(serverCfg)

	return wl
}

func (wl *WorkerLoop) configureExecutionConcurrency() {
	concurrencyLimit := wl.config.MaxConcurrency
	if concurrencyLimit <= 1 {
		concurrencyLimit = 1
	}
	wl.semaphore = parallel.NewTaskSemaphore(concurrencyLimit)
	if wl.config.WorktreeIsolation && concurrencyLimit > 1 {
		wl.worktreeManager = parallel.NewWorktreeManager(wl.config.WorkDir)
	}
}

// Start connects to the backend and begins processing tasks.
// @AX:ANCHOR[AUTO]: public lifecycle entry point — Start/Close are the primary WorkerLoop API; callers (CLI, tests) depend on error contract
func (wl *WorkerLoop) Start(ctx context.Context) error {
	// Acquire PID lock before starting to enforce single-instance constraint.
	wl.pidLock = pidlock.New(pidlock.DefaultPath())
	if err := wl.pidLock.Acquire(); err != nil {
		return fmt.Errorf("acquire PID lock: %w", err)
	}

	log.Printf("[worker] starting loop: provider=%s backend=%s", wl.config.Provider.Name(), wl.config.BackendURL)
	if err := wl.server.Start(ctx); err != nil {
		if releaseErr := wl.pidLock.Release(); releaseErr != nil {
			log.Printf("[worker] PID lock release failed on start error: %v", releaseErr)
		}
		return err
	}
	wl.startServices(ctx)

	wl.configureExecutionConcurrency()

	return nil
}

// Close shuts down the worker loop and its A2A server.
func (wl *WorkerLoop) Close() error {
	wl.stopServices()
	if wl.pidLock != nil {
		if err := wl.pidLock.Release(); err != nil {
			log.Printf("[worker] PID lock release failed: %v", err)
		}
	}
	return wl.server.Close()
}

// taskPayloadMessage is the JSON structure received from the A2A backend.
type taskPayloadMessage struct {
	Description             string                  `json:"description"`
	Prompt                  string                  `json:"prompt,omitempty"`
	PMNotes                 string                  `json:"pm_notes,omitempty"`
	PolicySummary           string                  `json:"policy_summary,omitempty"`
	KnowledgeCtx            string                  `json:"knowledge_ctx,omitempty"`
	PipelinePhases          []string                `json:"pipeline_phases,omitempty"`
	PipelineInstructions    map[string]string       `json:"pipeline_instructions,omitempty"`
	PipelinePromptTemplates map[string]string       `json:"pipeline_prompt_templates,omitempty"`
	IterationBudget         *budget.IterationBudget `json:"iteration_budget,omitempty"`
	SpecID                  string                  `json:"spec_id,omitempty"`
	Model                   string                  `json:"model,omitempty"`
	SessionID               string                  `json:"session_id,omitempty"`
}

// handleTask is the A2A TaskHandler callback invoked when a task is received.
func (wl *WorkerLoop) handleTask(ctx context.Context, taskID string, payload json.RawMessage) (*a2a.TaskResult, error) {
	log.Printf("[worker] received task: %s", taskID)

	// Clean up cached SecurityPolicy file on task completion (success or failure).
	defer cleanupPolicy(taskID)

	var msg taskPayloadMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("parse task payload: %w", err)
	}
	descriptionSeed := strings.TrimSpace(msg.Description)
	if descriptionSeed == "" {
		descriptionSeed = strings.TrimSpace(msg.Prompt)
	}
	memoryAgentID := resolveMemoryAgentID(wl.config)

	// Populate knowledge context from local Hub when backend did not provide one.
	knowledgeCtx := msg.KnowledgeCtx
	if knowledgeCtx == "" && wl.knowledgeSearcher != nil && descriptionSeed != "" {
		knowledgeCtx = populateKnowledge(ctx, wl.knowledgeSearcher, descriptionSeed)
	}

	// Populate memory context (SPEC-KHINT-001 REQ-003).
	memoryCtx := populateMemory(ctx, wl.memorySearcher, memoryAgentID, descriptionSeed)

	prompt := strings.TrimSpace(msg.Prompt)
	if prompt == "" {
		// Legacy/task-queue payloads still send structured fields that must be
		// assembled into the layer-4 prompt locally.
		prompt = wl.builder.Build(TaskPayload{
			TaskID:        taskID,
			Description:   msg.Description,
			PMNotes:       msg.PMNotes,
			PolicySummary: msg.PolicySummary,
			KnowledgeCtx:  knowledgeCtx,
			MemoryCtx:     memoryCtx,
			SpecID:        msg.SpecID,
		})
	}

	// Prefer the server-selected model when present. In signed control-plane
	// mode, local routing fallback is disabled so the worker stays thin.
	var model string
	if msg.Model != "" {
		model = msg.Model
	} else if wl.config.Router != nil && !a2a.SignedControlPlaneEnforced() {
		model = wl.config.Router.Route(wl.config.Provider.Name(), descriptionSeed)
	}

	// Configure the subprocess task.
	taskCfg := adapter.TaskConfig{
		TaskID:    taskID,
		SessionID: msg.SessionID,
		Prompt:    prompt,
		MCPConfig: wl.config.MCPConfig,
		WorkDir:   wl.config.WorkDir,
		Model:     model,
	}
	budgetCfg := budgetConfigFromMessage(msg)

	phasePlan, err := ParsePhasePlan(msg.PipelinePhases)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline phases: %w", err)
	}
	phaseInstructions, err := ParsePhaseInstructions(msg.PipelineInstructions)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline instructions: %w", err)
	}
	phasePromptTemplates, err := ParsePhasePromptTemplates(msg.PipelinePromptTemplates)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline prompt templates: %w", err)
	}

	// Execute subprocess with semaphore gating, worktree isolation, and audit recording.
	var result adapter.TaskResult
	if len(phasePlan) > 0 || len(phaseInstructions) > 0 || len(phasePromptTemplates) > 0 {
		result, err = wl.executePipelineWithParallel(ctx, taskID, prompt, model, phasePlan, phaseInstructions, phasePromptTemplates, budgetCfg)
	} else {
		result, err = wl.executeWithParallel(ctx, taskCfg, budgetCfg)
	}
	if err != nil {
		log.Printf("[worker] task %s failed: %v", taskID, err)
		return nil, err
	}

	log.Printf("[worker] task %s completed: cost=$%.4f duration=%dms", taskID, result.CostUSD, result.DurationMS)
	result.Artifacts = ensureOutputArtifact(result.Output, result.Artifacts)

	// Memory write-back: record task learnings (SPEC-KHINT-001 REQ-005).
	if wl.memorySearcher != nil && memoryAgentID != "" && result.Output != "" {
		go func() {
			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := wl.memorySearcher.CreateMemory(writeCtx, knowledge.CreateMemoryRequest{
				AgentID: memoryAgentID,
				Title:   fmt.Sprintf("Task learning: %s", taskID),
				Content: truncateForMemory(descriptionSeed, result.Output),
				Source:  "agent_learning",
			})
			if err != nil {
				log.Printf("[worker] memory write-back failed: %v", err)
			}
		}()
	}

	return &a2a.TaskResult{
		Status:    a2a.StatusCompleted,
		Artifacts: convertArtifacts(result.Artifacts),
	}, nil
}

// cleanupPolicy removes the cached SecurityPolicy file for the given task.
func cleanupPolicy(taskID string) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("autopus-%d", os.Getuid()))
	path := filepath.Join(dir, fmt.Sprintf("autopus-policy-%s.json", taskID))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("[worker] cleanup policy file: %v", err)
	}
}

// SetTUIProgram registers the bubbletea program for sending approval messages.
func (wl *WorkerLoop) SetTUIProgram(p *tea.Program) {
	wl.tuiProgram = p
}

// handleApproval forwards an approval request from A2A to the TUI.
func (wl *WorkerLoop) handleApproval(params a2a.ApprovalRequestParams) {
	if wl.tuiProgram == nil {
		log.Printf("[worker] approval request but no TUI program registered")
		return
	}
	wl.tuiProgram.Send(tui.ApprovalRequestMsg{
		TaskID:    params.TaskID,
		Action:    params.Action,
		RiskLevel: params.RiskLevel,
		Context:   params.Context,
	})
}

// SetOnApprovalDecision returns a callback that sends approval decisions to the backend.
func (wl *WorkerLoop) SetOnApprovalDecision() func(taskID, decision string) {
	return func(taskID, decision string) {
		if err := wl.server.SendApprovalResponse(taskID, decision); err != nil {
			log.Printf("[worker] send approval response error: %v", err)
		}
	}
}

// convertArtifacts converts adapter artifacts to A2A artifacts.
func convertArtifacts(src []adapter.Artifact) []a2a.Artifact {
	if len(src) == 0 {
		return nil
	}
	out := make([]a2a.Artifact, len(src))
	for i, a := range src {
		out[i] = a2a.Artifact{
			Name:     a.Name,
			MimeType: a.MimeType,
			Data:     a.Data,
		}
	}
	return out
}

func ensureOutputArtifact(output string, artifacts []adapter.Artifact) []adapter.Artifact {
	if strings.TrimSpace(output) == "" {
		return artifacts
	}
	for _, artifact := range artifacts {
		if artifact.Name == "output" {
			return artifacts
		}
	}
	return append([]adapter.Artifact{{
		Name:     "output",
		MimeType: "text/plain",
		Data:     output,
	}}, artifacts...)
}
