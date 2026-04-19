package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/security"
)

func (wl *WorkerLoop) detachedTaskContext(parent context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if parent != nil {
		base = context.WithoutCancel(parent)
	}
	ctx, cancel := context.WithCancel(base)

	if wl.lifecycleCtx != nil {
		go func() {
			select {
			case <-wl.lifecycleCtx.Done():
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	if parent != nil {
		go func() {
			select {
			case <-parent.Done():
				if errors.Is(parent.Err(), context.Canceled) {
					cancel()
				}
			case <-ctx.Done():
			}
		}()
	}

	return ctx, cancel
}

func (wl *WorkerLoop) executionContext(parent context.Context, taskID string) (context.Context, context.CancelFunc) {
	baseCtx, baseCancel := wl.detachedTaskContext(parent)
	deadline, ok := wl.taskExecutionDeadline(taskID)
	if !ok {
		return baseCtx, baseCancel
	}

	execCtx, timeoutCancel := context.WithDeadline(baseCtx, deadline)
	return execCtx, func() {
		timeoutCancel()
		baseCancel()
	}
}

func (wl *WorkerLoop) taskExecutionTimeout(taskID string) time.Duration {
	if strings.TrimSpace(taskID) == "" {
		return 0
	}

	policy, err := security.NewPolicyCache().Read(taskID)
	if err != nil || policy == nil || policy.TimeoutSec <= 0 {
		return 0
	}
	return time.Duration(policy.TimeoutSec) * time.Second
}

// StdinWriter wraps an io.WriteCloser to keep the stdin pipe open
// after the initial prompt is written. This enables mid-session
// message injection (e.g., budget warnings).
type StdinWriter struct {
	pipe io.WriteCloser
}

// NewStdinWriter creates a StdinWriter wrapping the given pipe.
func NewStdinWriter(pipe io.WriteCloser) *StdinWriter {
	return &StdinWriter{pipe: pipe}
}

// WritePrompt sends the initial prompt to the subprocess stdin.
// Unlike the previous implementation, the pipe is NOT closed after writing.
func (sw *StdinWriter) WritePrompt(prompt string) error {
	_, err := io.Copy(sw.pipe, strings.NewReader(prompt))
	return err
}

// Write implements io.Writer for injecting messages into stdin.
func (sw *StdinWriter) Write(p []byte) (int, error) {
	return sw.pipe.Write(p)
}

// Close closes the underlying pipe.
func (sw *StdinWriter) Close() error {
	return sw.pipe.Close()
}

func recordAuditEvent(w io.Writer, evt AuditEvent, logger LogBuffer) {
	if err := writeResilientAuditEvent(w, evt, logger); err != nil {
		log.Printf("[worker] audit event write failed: %v", err)
	}
}

// BudgetConfig holds optional budget configuration for subprocess execution.
type BudgetConfig struct {
	Budget        budget.IterationBudget
	EmergencyStop *security.EmergencyStop
}

// executeWithParallel wraps executeSubprocess with semaphore gating, worktree
// isolation, and audit event recording. It is the primary execution entry point
// called from handleTask.
func (wl *WorkerLoop) executeWithParallel(ctx context.Context, taskCfg adapter.TaskConfig, bc *BudgetConfig) (adapter.TaskResult, error) {
	taskID := taskCfg.TaskID
	startTime := time.Now()
	baseline := captureExecutionBaseline(taskCfg.WorkDir)

	// Record task start in the audit log.
	if wl.auditWriter != nil {
		recordAuditEvent(wl.auditWriter, newAuditStartedEvent(taskID, taskCfg.ComputerUse), wl.auditLogger)
	}

	// Acquire a semaphore slot when parallel execution is configured.
	// This blocks until a slot is available or ctx is cancelled.
	if wl.semaphore != nil {
		acquireCtx, cancelAcquire := wl.executionContext(ctx, taskID)
		defer cancelAcquire()
		if err := wl.semaphore.Acquire(acquireCtx); err != nil {
			return adapter.TaskResult{}, fmt.Errorf("acquire semaphore: %w", err)
		}
		defer wl.semaphore.Release()
	}

	// Create an isolated worktree when worktree isolation is enabled.
	// Falls back to the configured WorkDir on creation failure.
	if wl.worktreeManager != nil && wl.config.WorktreeIsolation {
		wtPath, err := wl.worktreeManager.Create(taskID)
		if err != nil {
			log.Printf("[worker] worktree create failed for %s, falling back to in-place: %v", taskID, err)
		} else {
			taskCfg.WorkDir = wtPath
			if prepErr := prepareSymphonyWorkspace(taskCfg.WorkDir, taskCfg.Prompt); prepErr != nil {
				log.Printf("[worker] symphony workspace prepare failed for %s: %v", taskID, prepErr)
			}
			if envErr := prepareTaskRuntimeEnv(&taskCfg); envErr != nil {
				log.Printf("[worker] runtime env prepare failed for %s: %v", taskID, envErr)
			}
			defer func() {
				removed, rmErr := wl.worktreeManager.RemoveIfClean(wtPath)
				if rmErr != nil {
					log.Printf("[worker] worktree remove failed: %v", rmErr)
					return
				}
				if !removed {
					log.Printf("[worker] preserving dirty worktree for %s: %s", taskID, wtPath)
				}
			}()
		}
	}

	// Delegate to the core subprocess executor.
	execCtx, cancelExec := wl.executionContext(ctx, taskID)
	defer cancelExec()
	result, err := wl.executeWithBudget(execCtx, taskCfg, bc)
	durationMS := time.Since(startTime).Milliseconds()

	// Record completion or failure in the audit log.
	if err != nil {
		if wl.auditWriter != nil {
			recordAuditEvent(wl.auditWriter, newAuditFailedEvent(taskID, durationMS, taskCfg.ComputerUse), wl.auditLogger)
		}
		return result, err
	}
	artifact, verifyErr := verifyExecutionPostconditions(taskCfg.WorkDir, taskCfg.Prompt, baseline)
	if artifact.Name != "" {
		result.Artifacts = append(result.Artifacts, artifact)
	}
	if verifyErr != nil {
		if wl.auditWriter != nil {
			recordAuditEvent(wl.auditWriter, newAuditFailedEvent(taskID, durationMS, taskCfg.ComputerUse), wl.auditLogger)
		}
		return result, verifyErr
	}
	if wl.auditWriter != nil {
		recordAuditEvent(wl.auditWriter, newAuditCompletedEvent(taskID, durationMS, result.CostUSD, taskCfg.ComputerUse), wl.auditLogger)
	}

	return result, nil
}

func (wl *WorkerLoop) executePipelineWithParallel(ctx context.Context, taskID, prompt, model string, phases []Phase, instructions map[Phase]string, promptTemplates map[Phase]string, bc *BudgetConfig) (adapter.TaskResult, error) {
	startTime := time.Now()

	if wl.auditWriter != nil {
		recordAuditEvent(wl.auditWriter, newAuditStartedEvent(taskID, false), wl.auditLogger)
	}

	if wl.semaphore != nil {
		acquireCtx, cancelAcquire := wl.executionContext(ctx, taskID)
		defer cancelAcquire()
		if err := wl.semaphore.Acquire(acquireCtx); err != nil {
			return adapter.TaskResult{}, fmt.Errorf("acquire semaphore: %w", err)
		}
		defer wl.semaphore.Release()
	}

	workDir := wl.config.WorkDir
	var envVars map[string]string
	if wl.worktreeManager != nil && wl.config.WorktreeIsolation {
		wtPath, err := wl.worktreeManager.Create(taskID)
		if err != nil {
			log.Printf("[worker] worktree create failed for %s, falling back to in-place: %v", taskID, err)
		} else {
			workDir = wtPath
			if prepErr := prepareSymphonyWorkspace(workDir, prompt); prepErr != nil {
				log.Printf("[worker] symphony workspace prepare failed for %s: %v", taskID, prepErr)
			}
			runtimeCfg := adapter.TaskConfig{TaskID: taskID, WorkDir: workDir}
			if envErr := prepareTaskRuntimeEnv(&runtimeCfg); envErr != nil {
				log.Printf("[worker] runtime env prepare failed for %s: %v", taskID, envErr)
			} else {
				envVars = runtimeCfg.EnvVars
			}
			defer func() {
				removed, rmErr := wl.worktreeManager.RemoveIfClean(wtPath)
				if rmErr != nil {
					log.Printf("[worker] worktree remove failed: %v", rmErr)
					return
				}
				if !removed {
					log.Printf("[worker] preserving dirty worktree for %s: %s", taskID, wtPath)
				}
			}()
		}
	}

	pe := NewPipelineExecutor(wl.config.Provider, wl.config.MCPConfig, workDir)
	baseline := captureExecutionBaseline(workDir)
	pe.SetEnvVars(envVars)
	pe.SetPhaseInstructions(instructions)
	pe.SetPhasePromptTemplates(promptTemplates)
	if bc != nil && bc.Budget.Limit > 0 {
		pe.SetIterationBudget(bc.Budget)
	}
	execCtx, cancelExec := wl.executionContext(ctx, taskID)
	defer cancelExec()
	result, err := pe.ExecuteWithPlan(execCtx, taskID, prompt, model, phases)
	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wl.auditWriter != nil {
			recordAuditEvent(wl.auditWriter, newAuditFailedEvent(taskID, durationMS, false), wl.auditLogger)
		}
		return adapter.TaskResult{}, err
	}
	artifact, verifyErr := verifyExecutionPostconditions(workDir, prompt, baseline)
	if artifact.Name != "" {
		result.Artifacts = append(result.Artifacts, artifact)
	}
	if verifyErr != nil {
		if wl.auditWriter != nil {
			recordAuditEvent(wl.auditWriter, newAuditFailedEvent(taskID, durationMS, false), wl.auditLogger)
		}
		return result, verifyErr
	}
	if wl.auditWriter != nil {
		recordAuditEvent(wl.auditWriter, newAuditCompletedEvent(taskID, durationMS, result.CostUSD, false), wl.auditLogger)
	}

	return result, nil
}
