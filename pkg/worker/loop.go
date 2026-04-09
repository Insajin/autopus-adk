package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/insajin/autopus-adk/pkg/worker/auth"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	workerNet "github.com/insajin/autopus-adk/pkg/worker/net"
	"github.com/insajin/autopus-adk/pkg/worker/parallel"
	"github.com/insajin/autopus-adk/pkg/worker/poll"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
	"github.com/insajin/autopus-adk/pkg/worker/tui"
)

// LoopConfig holds configuration for the WorkerLoop.
type LoopConfig struct {
	BackendURL      string
	WorkerName      string
	Skills          []string
	Provider        adapter.ProviderAdapter
	MCPConfig       string // path to worker-mcp.json
	WorkDir         string // working directory for subprocesses
	AuthToken       string          // bearer token for backend auth
	Router          *routing.Router // optional model router (nil = no routing)
	// Deprecated: use CredentialStore instead. Kept for backward compatibility.
	CredentialsPath string             // path to credentials.json for token refresh
	CredentialStore setup.CredentialStore // Secure credential storage (Keychain/encrypted file). If nil and CredentialsPath is set, falls back to plain file mode.
	AuditLogPath      string        // audit log file path (default: {WorkDir}/.autopus/audit.jsonl)
	AuditMaxSize      int64         // max log size before rotation (default: 10MB)
	AuditMaxAge       time.Duration // max age of rotated files (default: 7 days)
	WorkspaceID       string        // workspace identifier for scheduler
	MaxConcurrency    int           // max parallel tasks (0 or 1 = sequential)
	WorktreeIsolation bool          // enable worktree isolation for parallel tasks
	KnowledgeSync     bool          // enable knowledge file sync
	KnowledgeDir      string        // directory to watch for knowledge sync (defaults to WorkDir)
	KnowledgeSourceID string        // knowledge source ID for bridge binding
}

// WorkerLoop integrates A2A Server, ProviderAdapter, ContextBuilder, and StreamParser.
// It receives tasks via A2A, builds prompts, spawns CLI subprocesses, and reports results.
type WorkerLoop struct {
	config          LoopConfig
	server          *a2a.Server
	builder         ContextBuilder
	tuiProgram      *tea.Program
	authRefresher   *auth.TokenRefresher
	authReconnector *auth.Reconnector
	netMonitor      *workerNet.NetMonitor
	pollFallback    *poll.TaskPoller
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	auditWriter       *audit.RotatingWriter
	knowledgeSearcher *knowledge.KnowledgeSearcher
	knowledgeSyncer   *knowledge.Syncer
	knowledgeWatcher  *knowledge.FileWatcher
	semaphore         *parallel.TaskSemaphore
	worktreeManager   *parallel.WorktreeManager
	auditLogger       *slogAuditLogger
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

// Start connects to the backend and begins processing tasks.
func (wl *WorkerLoop) Start(ctx context.Context) error {
	log.Printf("[worker] starting loop: provider=%s backend=%s", wl.config.Provider.Name(), wl.config.BackendURL)
	if err := wl.server.Start(ctx); err != nil {
		return err
	}
	wl.startServices(ctx)

	// Initialize parallel execution components if concurrency limit is configured.
	if wl.config.MaxConcurrency > 1 {
		wl.semaphore = parallel.NewTaskSemaphore(wl.config.MaxConcurrency)
		if wl.config.WorktreeIsolation {
			wl.worktreeManager = parallel.NewWorktreeManager(wl.config.WorkDir)
		}
	}

	return nil
}

// Close shuts down the worker loop and its A2A server.
func (wl *WorkerLoop) Close() error {
	wl.stopServices()
	return wl.server.Close()
}

// taskPayloadMessage is the JSON structure received from the A2A backend.
type taskPayloadMessage struct {
	Description   string `json:"description"`
	PMNotes       string `json:"pm_notes,omitempty"`
	PolicySummary string `json:"policy_summary,omitempty"`
	KnowledgeCtx  string `json:"knowledge_ctx,omitempty"`
	SpecID        string `json:"spec_id,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
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

	// Populate knowledge context from local Hub when backend did not provide one.
	knowledgeCtx := msg.KnowledgeCtx
	if knowledgeCtx == "" && wl.knowledgeSearcher != nil {
		knowledgeCtx = populateKnowledge(ctx, wl.knowledgeSearcher, msg.Description)
	}

	// Build Layer 4 prompt via ContextBuilder.
	prompt := wl.builder.Build(TaskPayload{
		TaskID:        taskID,
		Description:   msg.Description,
		PMNotes:       msg.PMNotes,
		PolicySummary: msg.PolicySummary,
		KnowledgeCtx:  knowledgeCtx,
		SpecID:        msg.SpecID,
	})

	// Resolve model via router if configured (REQ-ROUTE-01).
	var model string
	if wl.config.Router != nil {
		model = wl.config.Router.Route(wl.config.Provider.Name(), msg.Description)
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

	// Execute subprocess with semaphore gating, worktree isolation, and audit recording.
	result, err := wl.executeWithParallel(ctx, taskCfg)
	if err != nil {
		log.Printf("[worker] task %s failed: %v", taskID, err)
		return nil, err
	}

	log.Printf("[worker] task %s completed: cost=$%.4f duration=%dms", taskID, result.CostUSD, result.DurationMS)

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
