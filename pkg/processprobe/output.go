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

// DefaultWaitDelay는 프로세스가 종료한 뒤 상속된 파이프를 배수할 유예이다.
//
// 이 값이 재는 대상은 "매달린 손자 프로세스"이며, "리더 고루틴이 스케줄되기까지
// 걸리는 시간"이 아니다. 250ms였을 때 두 성질이 뒤섞였다. `go test ./...`가
// 패키지를 코어 수만큼 병렬로 돌리면 종료 직후의 배수가 250ms를 넘기고,
// exec는 ErrWaitDelay로 출력을 잘라낸다. 그래서 타이밍 단정이 전혀 없는
// TestDetectBinaryFastVersion 같은 테스트까지 부하에서만 깨졌다.
//
// 2초는 여전히 상한이다. 매달린 파이프는 사용자가 체감하기 전에 끊기고,
// 이 값을 쓰는 모든 테스트의 elapsed 단정(probeFixtureLifetime/3 = 10초)
// 안쪽에 넉넉히 들어간다. 소유자는 이 패키지 하나이며, 호출자는 값을
// 재기술하지 말고 이 상수를 전달한다.
const DefaultWaitDelay = 2 * time.Second

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
		cmd.WaitDelay = DefaultWaitDelay
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
		cmd.WaitDelay = DefaultWaitDelay
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
