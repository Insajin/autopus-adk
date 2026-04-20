package worker

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/insajin/autopus-adk/pkg/worker/auth"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	workerNet "github.com/insajin/autopus-adk/pkg/worker/net"
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
					wl.updateAuthToken(newToken)
					log.Printf("[worker] auth token refreshed")
				},
			)
			go wl.authRefresher.Start(wl.lifecycleCtx)
			wl.authReconnector = auth.NewReconnector(wl.authRefresher, wl.server)
		}
	}

	// 3. Local knowledge search: automatic file sync via the legacy bridge path
	// has been removed, so only backend search is initialized here.
	if wl.config.KnowledgeSync && wl.config.WorkspaceID != "" {
		wl.knowledgeSearcher = knowledge.NewKnowledgeSearcher(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
		)
		log.Printf("[worker] automatic knowledge file sync is disabled")
	}

	// 3b. Memory searcher: enabled alongside knowledge (SPEC-KHINT-001 REQ-003).
	if wl.config.WorkspaceID != "" {
		wl.memorySearcher = knowledge.NewMemorySearcher(
			wl.config.BackendURL,
			wl.config.AuthToken,
			wl.config.WorkspaceID,
		)
		if resolveMemoryAgentID(wl.config) == "" {
			log.Printf("[worker] memory context/write-back disabled: set memory_agent_id or use UUID WorkerName")
		}
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
		wl.schedulerDisp = d
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

	// 6. REST fallback poller: reuses the A2A dispatch path when WebSocket receive
	// is exhausted. This keeps fallback behavior aligned with normal task handling.
	wl.server.SetRESTPoller(a2a.NewRESTPoller(a2a.RESTPollerConfig{
		BackendURL: wl.config.BackendURL,
		WorkerID:   wl.config.WorkerName,
		AuthToken:  wl.config.AuthToken,
		TaskHandler: func(task a2a.PollResult) error {
			return wl.server.HandlePolledTask(wl.lifecycleCtx, task)
		},
		OnAuthError: func(statusCode int) {
			log.Printf("[worker] REST poll auth error: status=%d", statusCode)
			if wl.authReconnector != nil {
				if err := wl.authReconnector.Reconnect(wl.lifecycleCtx); err != nil {
					log.Printf("[worker] REST poll auth recovery failed: %v", err)
				}
			}
		},
	}))

	// 7. Zombie reaper: detect and reap zombie child processes (FR-PROC-04).
	// @AX:NOTE[AUTO]: magic constant — 30s reaper interval matches reaper.go default; keep in sync if default changes
	wl.zombieReaper = reaper.New(reaper.Config{Interval: 30 * time.Second})
	wl.zombieReaper.Start(wl.lifecycleCtx) //nolint:errcheck
}

func (wl *WorkerLoop) updateAuthToken(token string) {
	if token == "" {
		return
	}
	wl.config.AuthToken = token
	wl.server.SetAuthToken(token)
	if wl.knowledgeSearcher != nil {
		wl.knowledgeSearcher.SetAuthToken(token)
	}
	if wl.memorySearcher != nil {
		wl.memorySearcher.SetAuthToken(token)
	}
	if wl.schedulerDisp != nil {
		wl.schedulerDisp.SetAuthToken(token)
	}
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

// activateFallbackPoller logs when the server-side A2A fallback path is engaged.
// The actual poller lifecycle is managed by a2a.Server.messageLoop.
func (wl *WorkerLoop) activateFallbackPoller() {
	log.Printf("[worker] activating A2A REST fallback poller")
	wl.emitHostEvent(HostEvent{
		Type:    HostEventRuntimeDegraded,
		Message: "A2A REST fallback poller activated",
	})
}
