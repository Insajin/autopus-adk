package worker

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/insajin/autopus-adk/pkg/worker/auth"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	workerNet "github.com/insajin/autopus-adk/pkg/worker/net"
	"github.com/insajin/autopus-adk/pkg/worker/poll"
	"github.com/insajin/autopus-adk/pkg/worker/reaper"
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

	// 2. TokenRefresher + Reconnector: JWT mode only — API Key mode skips token refresh.
	// CredentialStore path (preferred): uses secure Keychain/encrypted-file storage.
	// CredentialsPath path (deprecated): plain JSON file, kept for backward compatibility.
	isAPIKeyMode := strings.HasPrefix(wl.config.AuthToken, "acos_worker_")
	if !isAPIKeyMode {
		if wl.config.CredentialStore != nil {
			wl.authRefresher = auth.NewTokenRefresher(
				wl.config.BackendURL,
				wl.config.CredentialStore,
				func() { log.Printf("[worker] re-authentication needed") },
				func(newToken string) {
					wl.server.SetAuthToken(newToken)
					log.Printf("[worker] auth token refreshed")
				},
			)
			go wl.authRefresher.Start(wl.lifecycleCtx)
			wl.authReconnector = auth.NewReconnector(wl.authRefresher, wl.server)
		}
	}

	// 3. Knowledge syncer + watcher: enabled when KnowledgeSync and WorkspaceID are set.
	if wl.config.KnowledgeSync && wl.config.WorkspaceID != "" {
		wl.knowledgeSyncer = knowledge.NewSyncer(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
			wl.config.KnowledgeSourceID,
		)
		wl.knowledgeSearcher = knowledge.NewKnowledgeSearcher(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
		)
		knowledgeDir := wl.config.KnowledgeDir
		if knowledgeDir == "" {
			knowledgeDir = wl.config.WorkDir
		}
		wl.knowledgeWatcher = startKnowledgeWatcher(
			wl.lifecycleCtx,
			wl.knowledgeSyncer,
			knowledgeDir,
		)
	}

	// 3b. Memory searcher: enabled alongside knowledge (SPEC-KHINT-001 REQ-003).
	if wl.config.WorkspaceID != "" {
		wl.memorySearcher = knowledge.NewMemorySearcher(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
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
	// When a Reconnector is available, use coordinated reconnect (token refresh + WS reconnect).
	// Otherwise fall back to direct transport reconnect (API Key mode or no CredentialStore).
	wl.netMonitor = workerNet.NewNetMonitor(
		func(oldAddrs, newAddrs []string) {
			log.Printf("[worker] network change detected, reconnecting")
			var err error
			if wl.authReconnector != nil {
				err = wl.authReconnector.Reconnect(wl.lifecycleCtx)
			} else {
				err = wl.server.ReconnectTransport(wl.lifecycleCtx)
			}
			if err != nil {
				log.Printf("[worker] reconnect failed: %v", err)
			}
		},
		func() error {
			// Validate connectivity by attempting a WebSocket reconnect.
			return wl.server.ReconnectTransport(wl.lifecycleCtx)
		},
	)
	wl.netMonitor.Start(wl.lifecycleCtx)

	// 6. Zombie reaper: detect and reap zombie child processes (FR-PROC-04).
	// @AX:NOTE[AUTO]: magic constant — 30s reaper interval matches reaper.go default; keep in sync if default changes
	wl.zombieReaper = reaper.New(reaper.Config{Interval: 30 * time.Second})
	wl.zombieReaper.Start(wl.lifecycleCtx) //nolint:errcheck
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
	// Wait for zombie reaper goroutine to exit cleanly.
	if wl.zombieReaper != nil {
		wl.zombieReaper.Wait() //nolint:errcheck
	}
}

// activateFallbackPoller starts REST polling when WebSocket reconnect fails.
// It is a no-op if the fallback poller is already active.
// @AX:WARN[AUTO]: goroutine without bounded lifetime — fallback poller goroutine runs until lifecycleCtx; no mechanism to stop it independently once activated
func (wl *WorkerLoop) activateFallbackPoller() {
	if wl.pollFallback != nil {
		return // already active
	}
	wl.pollFallback = poll.NewTaskPoller(
		wl.config.BackendURL,
		wl.config.AuthToken,
		wl.config.WorkspaceID,
		func(taskData []byte) {
			// @AX:TODO[AUTO]:CYCLE:1 forward polled task to the A2A server's handleSendMessage path — currently logs only, no task processing
			// TODO: forward polled task to the A2A server's handleSendMessage path.
			// Currently logs only — WebSocket is the primary task delivery path.
			log.Printf("[worker] fallback poller received task (%d bytes) — processing not yet implemented", len(taskData))
		},
	)
	go wl.pollFallback.Start(wl.lifecycleCtx)
}
