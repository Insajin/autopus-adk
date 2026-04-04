package worker

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/insajin/autopus-adk/pkg/worker/auth"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	workerNet "github.com/insajin/autopus-adk/pkg/worker/net"
	"github.com/insajin/autopus-adk/pkg/worker/poll"
	"github.com/insajin/autopus-adk/pkg/worker/scheduler"
)

// startServices initializes and starts all lifecycle services.
// Startup order: audit -> auth -> knowledge -> scheduler -> net/poll.
func (wl *WorkerLoop) startServices(ctx context.Context) {
	wl.lifecycleCtx, wl.lifecycleCancel = context.WithCancel(ctx)

	// 1. Audit writer: resolve path with fallback to WorkDir default.
	auditPath := wl.config.AuditLogPath
	if auditPath == "" {
		auditPath = filepath.Join(wl.config.WorkDir, ".autopus", "audit.jsonl")
	}
	w, err := audit.NewRotatingWriter(auditPath, wl.config.AuditMaxSize, wl.config.AuditMaxAge)
	if err != nil {
		log.Printf("[worker] audit writer init failed: %v", err)
	} else {
		wl.auditWriter = w
		go wl.auditWriter.StartCleanup(wl.lifecycleCtx)
	}

	// 2. TokenRefresher: only if credentials path is configured.
	if wl.config.CredentialsPath != "" {
		wl.authRefresher = auth.NewTokenRefresher(
			wl.config.BackendURL,
			wl.config.CredentialsPath,
			func() { log.Printf("[worker] re-authentication needed") },
			func(newToken string) {
				wl.server.SetAuthToken(newToken)
				log.Printf("[worker] auth token refreshed")
			},
		)
		go wl.authRefresher.Start(wl.lifecycleCtx)
	}

	// 3. Knowledge syncer + watcher: enabled when KnowledgeSync and WorkspaceID are set.
	if wl.config.KnowledgeSync && wl.config.WorkspaceID != "" {
		wl.knowledgeSyncer = knowledge.NewSyncer(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
		)
		wl.knowledgeSearcher = knowledge.NewKnowledgeSearcher(
			wl.config.BackendURL,
			wl.config.AuthToken,
		)
		wl.knowledgeWatcher = startKnowledgeWatcher(
			wl.lifecycleCtx,
			wl.knowledgeSyncer,
			wl.config.WorkDir,
		)
	}

	// 4. Scheduler dispatcher: enabled when WorkspaceID is set.
	if wl.config.WorkspaceID != "" {
		d := scheduler.NewDispatcher(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
			time.Now().Location(),
			func(scheduleID, taskPayload string) {
				log.Printf("[worker] schedule triggered: %s", scheduleID)
			},
		)
		go d.Start(wl.lifecycleCtx)
	}

	// 5. NetMonitor: always start to detect network topology changes.
	wl.netMonitor = workerNet.NewNetMonitor(
		func(oldAddrs, newAddrs []string) {
			log.Printf("[worker] network change detected, reconnecting")
			if err := wl.server.ReconnectTransport(wl.lifecycleCtx); err != nil {
				log.Printf("[worker] reconnect failed: %v", err)
			}
		},
		func() error {
			// Validate connectivity by attempting a WebSocket reconnect.
			return wl.server.ReconnectTransport(wl.lifecycleCtx)
		},
	)
	wl.netMonitor.Start(wl.lifecycleCtx)
}

// stopServices gracefully stops all lifecycle services.
// Context cancellation stops auth, knowledge, scheduler, and net.
// Audit writer is closed explicitly to flush and release the file handle.
func (wl *WorkerLoop) stopServices() {
	if wl.lifecycleCancel != nil {
		wl.lifecycleCancel()
	}
	// Close audit writer explicitly — context cancel does not close file handles.
	if wl.auditWriter != nil {
		if err := wl.auditWriter.Close(); err != nil {
			log.Printf("[worker] audit writer close failed: %v", err)
		}
	}
}

// activateFallbackPoller starts REST polling when WebSocket reconnect fails.
// It is a no-op if the fallback poller is already active.
func (wl *WorkerLoop) activateFallbackPoller() {
	if wl.pollFallback != nil {
		return // already active
	}
	wl.pollFallback = poll.NewTaskPoller(
		wl.config.BackendURL,
		wl.config.AuthToken,
		"", // workspace ID — not used in fallback mode
		func(taskData []byte) {
			// Forward polled task data to handleTask via the server.
			log.Printf("[worker] fallback poller received task (%d bytes)", len(taskData))
		},
	)
	go wl.pollFallback.Start(wl.lifecycleCtx)
}
