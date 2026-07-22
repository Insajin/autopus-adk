// Package processprobe runs short-lived subprocess probes without unbounded pipe waits.
package processprobe

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

const defaultWaitDelay = 250 * time.Millisecond

var outputCommand = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

var limitedOutputCommand = func(cmd *exec.Cmd, _ func() bool) error {
	return cmd.Run()
}

// ErrOutputLimit indicates that a captured probe stream exceeded its limit.
var ErrOutputLimit = errors.New("process probe output limit exceeded")

// Output captures cmd output and bounds inherited-pipe draining after process exit.
// Callers must construct cmd with a bounded context when the process itself may hang.
func Output(cmd *exec.Cmd) ([]byte, error) {
	configureProcessGroup(cmd)
	if cmd.WaitDelay <= 0 {
		cmd.WaitDelay = defaultWaitDelay
	}
	out, err := outputCommand(cmd)
	if err != nil {
		terminateProcessGroup(cmd)
	}
	return out, err
}

// OutputLimited captures stdout while bounding stdout and stderr independently.
// Both streams must be unset before the call. An overflow cancels the command
// and terminates its process group; returned stdout never exceeds maxBytes.
func OutputLimited(cmd *exec.Cmd, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("process probe output limit must be positive: %d", maxBytes)
	}
	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if cmd.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}

	configureProcessGroup(cmd)
	if cmd.WaitDelay <= 0 {
		cmd.WaitDelay = defaultWaitDelay
	}
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() { cancelProbeCommand(cmd) })
	}
	stdout := newLimitedProbeOutput(maxBytes, stop)
	stderr := newLimitedProbeOutput(maxBytes, stop)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := limitedOutputCommand(cmd, func() bool {
		return stdout.Exceeded() || stderr.Exceeded()
	})
	if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
		exitErr.Stderr = stderr.Bytes()
	}
	if stdout.Exceeded() || stderr.Exceeded() {
		if !errors.Is(err, ErrOutputLimit) {
			err = errors.Join(ErrOutputLimit, err)
		}
	}
	if err != nil {
		terminateProcessGroup(cmd)
	}
	return stdout.Bytes(), err
}

type limitedProbeOutput struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	exceeded bool
	stop     func()
}

func newLimitedProbeOutput(limit int, stop func()) *limitedProbeOutput {
	return &limitedProbeOutput{
		data:  make([]byte, 0, limit),
		limit: limit,
		stop:  stop,
	}
}

func (output *limitedProbeOutput) Write(data []byte) (int, error) {
	written := len(data)
	output.mu.Lock()
	remaining := output.limit - len(output.data)
	if remaining >= len(data) {
		output.data = append(output.data, data...)
		output.mu.Unlock()
		return written, nil
	}
	if remaining > 0 {
		output.data = append(output.data, data[:remaining]...)
	}
	output.exceeded = true
	output.mu.Unlock()
	output.stop()
	return written, ErrOutputLimit
}

func (output *limitedProbeOutput) Bytes() []byte {
	output.mu.Lock()
	defer output.mu.Unlock()
	return bytes.Clone(output.data)
}

func (output *limitedProbeOutput) Exceeded() bool {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.exceeded
}

func cancelProbeCommand(cmd *exec.Cmd) {
	if cmd.Cancel != nil {
		if err := cmd.Cancel(); err == nil {
			return
		}
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
