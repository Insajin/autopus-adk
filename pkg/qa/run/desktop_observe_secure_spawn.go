package run

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
)

type secureDesktopSpawnSpec struct {
	command      string
	arguments    []string
	environment  []string
	codeIdentity desktopCodeIdentity
	fileIdentity desktopFileIdentity
}

type secureDesktopProcess struct {
	pid    int
	stdin  *os.File
	stdout *os.File
	stderr *os.File
	wait   func() (*os.ProcessState, error)
}

type secureDesktopCommandResult struct {
	exitCode int
	stderr   []byte
	stdout   []byte
}

func runSecureDesktopCommand(
	ctx context.Context,
	spec secureDesktopSpawnSpec,
	input []byte,
) (secureDesktopCommandResult, error) {
	if err := ctx.Err(); err != nil {
		return secureDesktopCommandResult{}, err
	}
	child, err := startSecureDesktopProcess(spec)
	if err != nil {
		return secureDesktopCommandResult{}, errDesktopProviderUnavailable
	}
	stdout := newSecureDesktopOutput()
	stderr := newSecureDesktopOutput()
	readDone := make(chan error, 2)
	go copySecureDesktopOutput(child.stdout, stdout, readDone)
	go copySecureDesktopOutput(child.stderr, stderr, readDone)
	writeDone := make(chan error, 1)
	go func() {
		_, writeErr := child.stdin.Write(input)
		closeErr := child.stdin.Close()
		writeDone <- errors.Join(writeErr, closeErr)
	}()
	waitDone := make(chan secureDesktopWaitResult, 1)
	go func() {
		state, waitErr := child.wait()
		waitDone <- secureDesktopWaitResult{state: state, err: waitErr}
	}()
	var waited secureDesktopWaitResult
	var terminalErr error
	select {
	case waited = <-waitDone:
	case <-ctx.Done():
		secureDesktopKillProcessGroup(child.pid)
		waited = <-waitDone
		terminalErr = ctx.Err()
	case <-stdout.overflowed:
		secureDesktopKillProcessGroup(child.pid)
		waited = <-waitDone
	case <-stderr.overflowed:
		secureDesktopKillProcessGroup(child.pid)
		waited = <-waitDone
	}
	groupErr := secureDesktopReapProcessGroup(child.pid)
	writeErr := <-writeDone
	readErr := errors.Join(<-readDone, <-readDone)
	closeSecureDesktopFiles(child)
	if terminalErr != nil {
		return secureDesktopCommandResult{}, terminalErr
	}
	if waited.state == nil || (waited.err != nil && waited.state.ExitCode() < 0) ||
		readErr != nil || groupErr != nil {
		return secureDesktopCommandResult{}, errDesktopProviderUnavailable
	}
	if writeErr != nil && waited.state.Success() {
		return secureDesktopCommandResult{}, errDesktopProviderUnavailable
	}
	if stdout.overflow || stderr.overflow {
		return secureDesktopCommandResult{}, desktopobserveEnvelopeTooLarge()
	}
	return secureDesktopCommandResult{
		exitCode: waited.state.ExitCode(),
		stderr:   append([]byte(nil), stderr.buffer...),
		stdout:   append([]byte(nil), stdout.buffer...),
	}, nil
}

type secureDesktopWaitResult struct {
	state *os.ProcessState
	err   error
}

type secureDesktopOutput struct {
	buffer     []byte
	mu         sync.Mutex
	once       sync.Once
	overflow   bool
	overflowed chan struct{}
}

func newSecureDesktopOutput() *secureDesktopOutput {
	return &secureDesktopOutput{overflowed: make(chan struct{})}
}

func (output *secureDesktopOutput) Write(value []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	remaining := desktopObservationEnvelopeLimit() - len(output.buffer)
	if remaining > len(value) {
		remaining = len(value)
	}
	if remaining > 0 {
		output.buffer = append(output.buffer, value[:remaining]...)
	}
	if remaining < len(value) {
		output.overflow = true
		output.once.Do(func() { close(output.overflowed) })
	}
	return len(value), nil
}

func copySecureDesktopOutput(source *os.File, target io.Writer, done chan<- error) {
	_, err := io.Copy(target, source)
	done <- err
}

func closeSecureDesktopFiles(child *secureDesktopProcess) {
	if child == nil {
		return
	}
	for _, file := range []*os.File{child.stdin, child.stdout, child.stderr} {
		if file != nil {
			_ = file.Close()
		}
	}
}
