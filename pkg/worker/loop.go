package worker

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/insajin/autopus-adk/pkg/worker/auth"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	workerNet "github.com/insajin/autopus-adk/pkg/worker/net"
	"github.com/insajin/autopus-adk/pkg/worker/parallel"
	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
	"github.com/insajin/autopus-adk/pkg/worker/reaper"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
	"github.com/insajin/autopus-adk/pkg/worker/scheduler"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// LoopConfig holds configuration for the WorkerLoop.
type LoopConfig struct {
	BackendURL    string
	WorkerName    string
	MemoryAgentID string
	Skills        []string
	Providers     []string
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
	approvalMu        sync.Mutex
	pendingApprovals  map[string]a2a.ApprovalRequestParams
	observerMu        sync.RWMutex
	hostObservers     []HostObserver
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
		config:           config,
		pendingApprovals: make(map[string]a2a.ApprovalRequestParams),
		auditLogger:      newSlogAuditLogger(3),
	}

	serverCfg := a2a.ServerConfig{
		BackendURL:            config.BackendURL,
		WorkerName:            config.WorkerName,
		WorkspaceID:           config.WorkspaceID,
		Skills:                config.Skills,
		Providers:             config.Providers,
		Handler:               wl.handleTask,
		AuthToken:             config.AuthToken,
		ApprovalCallback:      wl.handleApproval,
		DispatchIssueCallback: wl.handleDispatchIssue,
		OnConnectionExhausted: wl.activateFallbackPoller,
	}
	wl.server = a2a.NewServer(serverCfg)

	return wl
}
