package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/insajin/autopus-adk/pkg/worker/stream"
)

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

	// Record task start in the audit log.
	if wl.auditWriter != nil {
		writeResilientAuditEvent(wl.auditWriter, newAuditStartedEvent(taskID, taskCfg.ComputerUse), wl.auditLogger)
	}

	// Acquire a semaphore slot when parallel execution is configured.
	// This blocks until a slot is available or ctx is cancelled.
	if wl.semaphore != nil {
		if err := wl.semaphore.Acquire(ctx); err != nil {
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
	result, err := wl.executeWithBudget(ctx, taskCfg, bc)
	durationMS := time.Since(startTime).Milliseconds()

	// Record completion or failure in the audit log.
	if err != nil {
		if wl.auditWriter != nil {
			writeResilientAuditEvent(wl.auditWriter, newAuditFailedEvent(taskID, durationMS, taskCfg.ComputerUse), wl.auditLogger)
		}
		return result, err
	}
	if wl.auditWriter != nil {
		writeResilientAuditEvent(wl.auditWriter, newAuditCompletedEvent(taskID, durationMS, result.CostUSD, taskCfg.ComputerUse), wl.auditLogger)
	}

	return result, nil
}

func (wl *WorkerLoop) executePipelineWithParallel(ctx context.Context, taskID, prompt, model string, phases []Phase, instructions map[Phase]string, promptTemplates map[Phase]string, bc *BudgetConfig) (adapter.TaskResult, error) {
	startTime := time.Now()

	if wl.auditWriter != nil {
		writeResilientAuditEvent(wl.auditWriter, newAuditStartedEvent(taskID, false), wl.auditLogger)
	}

	if wl.semaphore != nil {
		if err := wl.semaphore.Acquire(ctx); err != nil {
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
	pe.SetEnvVars(envVars)
	pe.SetPhaseInstructions(instructions)
	pe.SetPhasePromptTemplates(promptTemplates)
	if bc != nil && bc.Budget.Limit > 0 {
		pe.SetIterationBudget(bc.Budget)
	}
	result, err := pe.ExecuteWithPlan(ctx, taskID, prompt, model, phases)
	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wl.auditWriter != nil {
			writeResilientAuditEvent(wl.auditWriter, newAuditFailedEvent(taskID, durationMS, false), wl.auditLogger)
		}
		return adapter.TaskResult{}, err
	}
	if wl.auditWriter != nil {
		writeResilientAuditEvent(wl.auditWriter, newAuditCompletedEvent(taskID, durationMS, result.CostUSD, false), wl.auditLogger)
	}

	return result, nil
}

func prepareSymphonyWorkspace(workDir, prompt string) error {
	if !strings.Contains(prompt, ".symphony/prompt.md") {
		return nil
	}

	symphonyDir := filepath.Join(workDir, ".symphony")
	if err := os.MkdirAll(filepath.Join(symphonyDir, "artifacts"), 0o755); err != nil {
		return fmt.Errorf("create .symphony dir: %w", err)
	}

	promptPath := filepath.Join(symphonyDir, "prompt.md")
	if _, err := os.Stat(promptPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat prompt.md: %w", err)
	}

	if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
		return fmt.Errorf("write prompt.md: %w", err)
	}
	if err := os.Chmod(promptPath, 0o444); err != nil {
		return fmt.Errorf("chmod prompt.md: %w", err)
	}
	return nil
}

func prepareTaskRuntimeEnv(taskCfg *adapter.TaskConfig) error {
	symphonyDir := filepath.Join(taskCfg.WorkDir, ".symphony")
	artifactsDir := filepath.Join(symphonyDir, "artifacts")
	tmpDir := filepath.Join(artifactsDir, "tmp")
	goCacheDir := filepath.Join(artifactsDir, "gocache")

	for _, dir := range []string{artifactsDir, tmpDir, goCacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create runtime dir %s: %w", dir, err)
		}
	}

	if taskCfg.EnvVars == nil {
		taskCfg.EnvVars = make(map[string]string)
	}
	taskCfg.EnvVars["TMPDIR"] = tmpDir
	taskCfg.EnvVars["TEST_TMPDIR"] = tmpDir
	taskCfg.EnvVars["GOTMPDIR"] = tmpDir
	taskCfg.EnvVars["GOCACHE"] = goCacheDir

	return nil
}

func budgetConfigFromMessage(msg taskPayloadMessage) *BudgetConfig {
	if msg.IterationBudget == nil || msg.IterationBudget.Limit <= 0 {
		return nil
	}
	cloned := *msg.IterationBudget
	return &BudgetConfig{
		Budget:        cloned,
		EmergencyStop: security.NewEmergencyStop(),
	}
}

// executeSubprocess spawns the provider CLI, pipes the prompt via stdin,
// and reads stdout line-by-line through StreamParser.
func (wl *WorkerLoop) executeSubprocess(ctx context.Context, taskCfg adapter.TaskConfig) (adapter.TaskResult, error) {
	return wl.executeWithBudget(ctx, taskCfg, nil)
}

// executeWithBudget spawns the provider CLI with optional budget tracking.
// @WARN: When budget is exhausted, EmergencyStop.Stop() is called.
func (wl *WorkerLoop) executeWithBudget(ctx context.Context, taskCfg adapter.TaskConfig, bc *BudgetConfig) (adapter.TaskResult, error) {
	cmd := wl.config.Provider.BuildCommand(ctx, taskCfg)

	// Set up stdin pipe — kept open for budget warning injection (REQ-BUDGET-03).
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return adapter.TaskResult{}, fmt.Errorf("stdin pipe: %w", err)
	}
	sw := NewStdinWriter(stdinPipe)

	// Set up stdout pipe for stream parsing.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return adapter.TaskResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for diagnostics on non-zero exit (REQ-SUB-H01).
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	log.Printf("[worker] task %s: starting subprocess: %s %v (workdir=%s)", taskCfg.TaskID, cmd.Path, cmd.Args[1:], cmd.Dir)
	if err := cmd.Start(); err != nil {
		return adapter.TaskResult{}, fmt.Errorf("start subprocess: %w", err)
	}

	// Register for emergency stop if budget config provided.
	if bc != nil && bc.EmergencyStop != nil {
		bc.EmergencyStop.SetProcess(cmd)
		defer bc.EmergencyStop.ClearProcess()
	}

	// Write prompt and close stdin so claude --print can start processing.
	// NOTE: closing stdin disables mid-session budget warning injection.
	// A future iteration should use a separate signaling mechanism.
	taskID := taskCfg.TaskID
	if err := sw.WritePrompt(taskCfg.Prompt); err != nil {
		log.Printf("[worker] task %s: failed to write prompt: %v", taskID, err)
	}
	sw.Close() // EOF signals claude --print that prompt is complete

	// Parse stream output with optional budget tracking.
	result, parseErr := wl.parseStreamWithBudget(stdout, taskID, nil, bc)

	// Wait for process to exit.
	waitErr := cmd.Wait()
	if parseErr != nil {
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			log.Printf("[worker] task %s: stderr: %s", taskID, stderrStr)
		}
		return adapter.TaskResult{}, fmt.Errorf("parse stream: %w", parseErr)
	}
	if waitErr != nil {
		if result.Output != "" {
			log.Printf("[worker] task %s: exited with error but produced output: %v", taskID, waitErr)
			return result, nil
		}
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return adapter.TaskResult{}, fmt.Errorf("subprocess exit: %w\nstderr: %s", waitErr, stderrStr)
		}
		return adapter.TaskResult{}, fmt.Errorf("subprocess exit: %w", waitErr)
	}

	return result, nil
}

// parseStream reads subprocess stdout and extracts the final result (no budget).
func (wl *WorkerLoop) parseStream(r io.Reader, taskID string) (adapter.TaskResult, error) {
	return wl.parseStreamWithBudget(r, taskID, nil, nil)
}

// parseStreamWithBudget extends parseStream with tool call counting and warnings.
func (wl *WorkerLoop) parseStreamWithBudget(r io.Reader, taskID string, sw *StdinWriter, bc *BudgetConfig) (adapter.TaskResult, error) {
	scanner := bufio.NewScanner(r)
	var lastResult adapter.TaskResult
	hasResult := false

	// Set up budget counter and warning injector if configured.
	var counter *budget.Counter
	var injector *budget.WarningInjector
	if bc != nil {
		counter = budget.NewCounter(bc.Budget)
		if sw != nil {
			injector = budget.NewWarningInjector(sw)
		}
	}

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		adapterEvt, err := wl.config.Provider.ParseEvent(append([]byte(nil), line...))
		if err != nil {
			log.Printf("[stream] skipping malformed line: %v", err)
			continue
		}

		switch {
		case adapterEvt.Type == "system" && adapterEvt.Subtype == "init":
			log.Printf("[worker] task %s: subprocess initialized", taskID)

		case adapterEvt.Type == "system" && adapterEvt.Subtype == "task_started":
			log.Printf("[worker] task %s: subagent started", taskID)

		case adapterEvt.Type == "system" && adapterEvt.Subtype == "task_progress":
			log.Printf("[worker] task %s: progress update", taskID)

		case adapterEvt.Type == "system" && adapterEvt.Subtype == "task_notification":
			log.Printf("[worker] task %s: subagent notification", taskID)

		// REQ-BUDGET-01/05: Count tool_call and tool_use events.
		case adapterEvt.Type == stream.EventToolCall || adapterEvt.Type == "tool_use":
			if counter != nil {
				r := counter.Increment()
				log.Printf("[worker] task %s: tool call %d/%d", taskID, r.Count, r.Budget.Limit)

				// REQ-BUDGET-06/07: Inject warnings on threshold change.
				if injector != nil {
					injector.Inject(r)
				}

				// REQ-BUDGET-08: Hard limit — emergency stop.
				if r.Level == budget.LevelExhausted && bc.EmergencyStop != nil {
					log.Printf("[worker] task %s: budget exhausted, stopping", taskID)
					_ = bc.EmergencyStop.Stop("iteration_budget_exceeded")
					return lastResult, fmt.Errorf("iteration budget exceeded: %d/%d", r.Count, r.Budget.Limit)
				}
			}

		case adapterEvt.Type == "result":
			lastResult = wl.config.Provider.ExtractResult(adapterEvt)
			hasResult = true

		case adapterEvt.Type == "error":
			log.Printf("[worker] task %s: error event received", taskID)
		}
	}
	if err := scanner.Err(); err != nil {
		return adapter.TaskResult{}, fmt.Errorf("stream scan: %w", err)
	}

	if !hasResult {
		return adapter.TaskResult{}, fmt.Errorf("no result event received for task %s", taskID)
	}
	return lastResult, nil
}
