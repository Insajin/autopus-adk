package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/parallel"
	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
	"github.com/insajin/autopus-adk/pkg/worker/tui"
)

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
	for i, artifact := range src {
		out[i] = a2a.Artifact{
			Name:     artifact.Name,
			MimeType: artifact.MimeType,
			Data:     artifact.Data,
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
