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

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/insajin/autopus-adk/pkg/worker/stream"
)

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
	prepareCommandProcessGroup(cmd)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return adapter.TaskResult{}, fmt.Errorf("stdin pipe: %w", err)
	}
	sw := NewStdinWriter(stdinPipe)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return adapter.TaskResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	log.Printf("[worker] task %s: starting subprocess: %s %v (workdir=%s)", taskCfg.TaskID, cmd.Path, cmd.Args[1:], cmd.Dir)
	if err := cmd.Start(); err != nil {
		return adapter.TaskResult{}, fmt.Errorf("start subprocess: %w", err)
	}
	stopCancellationWatcher := watchCommandCancellation(ctx, cmd, taskCfg.TaskID)
	defer stopCancellationWatcher()

	if bc != nil && bc.EmergencyStop != nil {
		bc.EmergencyStop.SetProcess(cmd)
		defer bc.EmergencyStop.ClearProcess()
	}

	taskID := taskCfg.TaskID
	if err := sw.WritePrompt(taskCfg.Prompt); err != nil {
		log.Printf("[worker] task %s: failed to write prompt: %v", taskID, err)
	}
	sw.Close()

	result, parseErr := wl.parseStreamWithBudget(stdout, taskID, nil, bc)
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

func (wl *WorkerLoop) parseStream(r io.Reader, taskID string) (adapter.TaskResult, error) {
	return wl.parseStreamWithBudget(r, taskID, nil, nil)
}

func (wl *WorkerLoop) parseStreamWithBudget(r io.Reader, taskID string, sw *StdinWriter, bc *BudgetConfig) (adapter.TaskResult, error) {
	scanner := bufio.NewScanner(r)
	var lastResult adapter.TaskResult
	hasResult := false

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
		case adapterEvt.Type == stream.EventToolCall || adapterEvt.Type == "tool_use":
			if counter != nil {
				r := counter.Increment()
				log.Printf("[worker] task %s: tool call %d/%d", taskID, r.Count, r.Budget.Limit)
				if injector != nil {
					injector.Inject(r)
				}
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
