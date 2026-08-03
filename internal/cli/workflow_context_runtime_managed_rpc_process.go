package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type workflowContextManagedRPCProcess struct {
	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	frameCancel context.CancelFunc
	frames      <-chan []byte
	done        <-chan error
	stderr      *bytes.Buffer
	started     bool
	closed      bool
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: external process boundary for the isolated OMP stdio session.
// @AX:REASON [AUTO]: sandboxing, process-group ownership, bridge environment, and bounded shutdown must be established before RPC frames are accepted.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the 500 ms WaitDelay bounds pipe shutdown after context cancellation or process-group termination.
func startWorkflowContextManagedRPCProcess(
	ctx context.Context,
	options WorkflowContextManagedRPCOptions,
	binding WorkflowContextBridgeBinding,
) (*workflowContextManagedRPCProcess, bool, error) {
	args := []string{
		"--mode", "rpc", "--no-session", "--session-dir", options.SessionDir,
		"--cwd", options.Workspace, "--model", options.Model, "--config", options.ConfigPath,
		"--no-tools", "--no-lsp", "--no-pty", "--no-skills", "--no-rules", "--no-title",
		"--max-time", options.MaxTime.String(),
	}
	cmd := exec.CommandContext(ctx, options.Executable, args...)
	cmd.Dir = options.Workspace
	cmd.Env = workflowContextManagedRPCEnvironment(options.Environment, binding)
	cmd.WaitDelay = 500 * time.Millisecond
	sandboxed, err := configureWorkflowContextManagedRPCSandbox(cmd, options.AllowedEndpoint)
	if err != nil {
		return nil, false, err
	}
	if err := configureWorkflowContextManagedRPCProcessGroup(cmd); err != nil {
		return nil, false, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, false, fmt.Errorf("open managed OMP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, fmt.Errorf("open managed OMP stdout: %w", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("start managed OMP RPC: %w", err)
	}
	frameCtx, frameCancel := context.WithCancel(ctx)
	frames, done := workflowContextManagedRPCFrames(frameCtx, stdout)
	return &workflowContextManagedRPCProcess{
		cmd: cmd, stdin: stdin, stdout: stdout, frameCancel: frameCancel,
		frames: frames, done: done, stderr: stderr, started: true,
	}, sandboxed, nil
}

func (process *workflowContextManagedRPCProcess) Close() error {
	if process == nil {
		return nil
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	if !process.started || process.closed {
		return nil
	}
	if process.frameCancel != nil {
		process.frameCancel()
	}
	if process.stdin != nil {
		_ = process.stdin.Close()
	}
	if process.stdout != nil {
		_ = process.stdout.Close()
	}
	killErr := terminateWorkflowContextManagedRPCProcessGroup(process.cmd)
	waitErr := process.cmd.Wait()
	process.closed = true
	var exitErr *exec.ExitError
	if killErr != nil {
		return killErr
	}
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return waitErr
	}
	return nil
}

func (process *workflowContextManagedRPCProcess) Active() bool {
	process.mu.Lock()
	defer process.mu.Unlock()
	return process.started && !process.closed && workflowContextManagedRPCProcessActive(process.cmd)
}

func (process *workflowContextManagedRPCProcess) PID() int {
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.cmd == nil || process.cmd.Process == nil {
		return 0
	}
	return process.cmd.Process.Pid
}

func (process *workflowContextManagedRPCProcess) errorWithStderr(reason string) error {
	return errors.New(reason)
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the 10-second ACK timeout matches the embedded bridge maximum and keeps confirmation fail-closed.
func workflowContextManagedRPCEnvironment(
	base []string, binding WorkflowContextBridgeBinding,
) []string {
	result := make([]string, 0, len(base)+5)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if _, denied := workflowContextManagedReservedEnvironment[key]; !found || denied {
			continue
		}
		result = append(result, entry)
	}
	result = append(result,
		"AUTOPUS_OMP_CONTEXT_BINDING_HASH="+binding.BindingHash,
		"AUTOPUS_OMP_CONTEXT_OPTIONS_HASH="+binding.OptionsHash,
		"AUTOPUS_OMP_CONTEXT_SESSION_HASH="+binding.SessionHash,
		"AUTOPUS_OMP_CONTEXT_NONCE_HASH="+binding.NonceHash,
		"AUTOPUS_OMP_CONTEXT_ACK_TIMEOUT_MS="+strconv.Itoa(10000),
	)
	return result
}
