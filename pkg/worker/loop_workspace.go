package worker

import (
	"context"
	"log"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/workspace"
)

// WorkspaceConfig holds connection details for a single workspace.
type WorkspaceConfig struct {
	WorkspaceID string
	ProjectDir  string
	BackendURL  string
	AuthToken   string
}

// startWorkspaceGoroutines spawns per-workspace A2A server goroutines.
// Returns the MultiWorkspace manager for task routing.
func startWorkspaceGoroutines(ctx context.Context, workspaces []WorkspaceConfig, handler a2a.TaskHandler) *workspace.MultiWorkspace {
	mw := workspace.NewMultiWorkspace()
	for _, ws := range workspaces {
		conn := workspace.WorkspaceConn{
			WorkspaceID: ws.WorkspaceID,
			ProjectDir:  ws.ProjectDir,
			BackendURL:  ws.BackendURL,
			AuthToken:   ws.AuthToken,
			Connected:   true,
		}
		mw.Add(conn)

		go func(cfg WorkspaceConfig) {
			serverCfg := a2a.ServerConfig{
				BackendURL: cfg.BackendURL,
				WorkerName: cfg.WorkspaceID,
				Handler:    handler,
				AuthToken:  cfg.AuthToken,
			}
			srv := a2a.NewServer(serverCfg)
			if err := srv.Start(ctx); err != nil {
				log.Printf("[worker] workspace %s server start failed: %v", cfg.WorkspaceID, err)
				return
			}
			<-ctx.Done()
			if err := srv.Close(); err != nil {
				log.Printf("[worker] workspace %s server close failed: %v", cfg.WorkspaceID, err)
			}
		}(ws)
	}
	return mw
}

// routeWorkDir returns the project directory for a workspace-targeted task.
// Falls back to defaultDir when the multi-workspace manager is nil or not found.
func routeWorkDir(mw *workspace.MultiWorkspace, workspaceID, defaultDir string) string {
	if mw == nil || workspaceID == "" {
		return defaultDir
	}
	dir, err := mw.RouteTask(workspaceID)
	if err != nil {
		log.Printf("[worker] workspace route failed for %q: %v, using default", workspaceID, err)
		return defaultDir
	}
	return dir
}
