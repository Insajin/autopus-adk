//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

func supportsOMPReadinessBehaviorProcessGroup() bool {
	return true
}

// @AX:WARN [AUTO]: bounded RPC process supervision has cyclomatic complexity 16.
// @AX:REASON [AUTO]: gocyclo reports 16 across pipe setup, process-group termination, stream limits, terminal frames, and context cancellation.
func runOMPReadinessRPCCommand(
	ctx context.Context,
	cmd *exec.Cmd,
	input []byte,
	maxOutput int,
) ([]byte, error) {
	if maxOutput <= 0 {
		return nil, errors.New("OMP readiness output limit must be positive")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	limit := newOMPReadinessLimitSignal()
	capture := newOMPReadinessRPCStreamCapture(maxOutput, limit)
	stderrCapture := &ompReadinessCountWriter{limit: maxOutput, signal: limit}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 250 * time.Millisecond
	var terminateOnce sync.Once
	terminate := func() {
		terminateOnce.Do(func() {
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		})
	}
	cmd.Cancel = func() error {
		terminate()
		return nil
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(capture, stdout)
		close(stdoutDone)
	}()
	go func() {
		_, _ = io.Copy(stderrCapture, stderr)
		close(stderrDone)
	}()
	if _, err := stdin.Write(input); err != nil {
		_ = stdin.Close()
		terminate()
		_ = cmd.Wait()
		<-stdoutDone
		<-stderrDone
		return capture.Bytes(), err
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	terminal := capture.Terminal()
	for {
		select {
		case waitErr := <-wait:
			_ = stdin.Close()
			<-stdoutDone
			<-stderrDone
			if waitErr != nil {
				terminate()
			}
			if capture.Exceeded() || stderrCapture.Exceeded() {
				return capture.Bytes(), errors.Join(processprobe.ErrOutputLimit, waitErr)
			}
			return capture.Bytes(), waitErr
		case <-terminal:
			_ = stdin.Close()
			terminal = nil
		case <-limit.Done():
			terminate()
			waitErr := <-wait
			<-stdoutDone
			<-stderrDone
			return capture.Bytes(), errors.Join(processprobe.ErrOutputLimit, waitErr)
		case <-ctx.Done():
			terminate()
			waitErr := <-wait
			<-stdoutDone
			<-stderrDone
			return capture.Bytes(), errors.Join(ctx.Err(), waitErr)
		}
	}
}

type ompReadinessLimitSignal struct {
	once sync.Once
	done chan struct{}
}

func newOMPReadinessLimitSignal() *ompReadinessLimitSignal {
	return &ompReadinessLimitSignal{done: make(chan struct{})}
}

func (signal *ompReadinessLimitSignal) Notify() {
	signal.once.Do(func() { close(signal.done) })
}

func (signal *ompReadinessLimitSignal) Done() <-chan struct{} {
	return signal.done
}

type ompReadinessCountWriter struct {
	mu       sync.Mutex
	written  int
	limit    int
	exceeded bool
	signal   *ompReadinessLimitSignal
}

func (writer *ompReadinessCountWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(data) > writer.limit-writer.written {
		writer.exceeded = true
		writer.signal.Notify()
		return len(data), processprobe.ErrOutputLimit
	}
	writer.written += len(data)
	return len(data), nil
}

func (writer *ompReadinessCountWriter) Exceeded() bool {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.exceeded
}

type ompReadinessRPCStreamCapture struct {
	mu         sync.Mutex
	data       []byte
	limit      int
	exceeded   bool
	scanOffset int
	starts     map[string]bool
	paired     bool
	terminal   chan struct{}
	termOnce   sync.Once
	signal     *ompReadinessLimitSignal
}

func newOMPReadinessRPCStreamCapture(
	limit int,
	signal *ompReadinessLimitSignal,
) *ompReadinessRPCStreamCapture {
	return &ompReadinessRPCStreamCapture{
		data: make([]byte, 0, limit), limit: limit, starts: make(map[string]bool),
		terminal: make(chan struct{}), signal: signal,
	}
}

func (capture *ompReadinessRPCStreamCapture) Write(data []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(data) > capture.limit-len(capture.data) {
		remaining := capture.limit - len(capture.data)
		capture.data = append(capture.data, data[:remaining]...)
		capture.exceeded = true
		capture.signal.Notify()
		return len(data), processprobe.ErrOutputLimit
	}
	capture.data = append(capture.data, data...)
	capture.scanFrames()
	return len(data), nil
}

// @AX:WARN [AUTO]: streaming RPC frame scanning has cyclomatic complexity 16.
// @AX:REASON [AUTO]: gocyclo reports 16 across partial-line buffering, JSON framing, request correlation, and terminal-state pairing.
func (capture *ompReadinessRPCStreamCapture) scanFrames() {
	for {
		relative := bytes.IndexByte(capture.data[capture.scanOffset:], '\n')
		if relative < 0 {
			return
		}
		end := capture.scanOffset + relative
		var frame struct {
			Type       string `json:"type"`
			Command    string `json:"command"`
			Success    bool   `json:"success"`
			ToolCallID string `json:"toolCallId"`
			Message    struct {
				Role string `json:"role"`
			} `json:"message"`
			Data struct {
				AgentInvoked *bool `json:"agentInvoked"`
			} `json:"data"`
		}
		if json.Unmarshal(capture.data[capture.scanOffset:end], &frame) == nil {
			switch frame.Type {
			case "tool_execution_start":
				capture.starts[frame.ToolCallID] = frame.ToolCallID != ""
			case "tool_execution_end":
				capture.paired = capture.paired || capture.starts[frame.ToolCallID]
			case "message_end":
				if capture.paired && frame.Message.Role == "assistant" {
					capture.signalTerminal()
				}
			case "agent_end":
				capture.signalTerminal()
			case "response":
				if frame.Command == "prompt" && (!frame.Success ||
					(frame.Data.AgentInvoked != nil && !*frame.Data.AgentInvoked)) {
					capture.signalTerminal()
				}
			}
		}
		capture.scanOffset = end + 1
	}
}

func (capture *ompReadinessRPCStreamCapture) signalTerminal() {
	capture.termOnce.Do(func() { close(capture.terminal) })
}

func (capture *ompReadinessRPCStreamCapture) Terminal() <-chan struct{} {
	return capture.terminal
}

func (capture *ompReadinessRPCStreamCapture) Bytes() []byte {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return bytes.Clone(capture.data)
}

func (capture *ompReadinessRPCStreamCapture) Exceeded() bool {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.exceeded
}
