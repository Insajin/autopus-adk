package orchestra

import (
	"context"
	"io"
	"os/exec"
)

// command는 실행 커맨드 인터페이스이다 (테스트 목 지원).
type command interface {
	StdinPipe() (io.WriteCloser, error)
	SetStdin(r io.Reader)
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	Start() error
	Wait() error
	ExitCode() int
}

// execCommand는 실제 exec.Cmd 래퍼이다.
type execCommand struct {
	cmd *exec.Cmd
}

// newCommand는 컨텍스트 기반 커맨드를 생성한다.
var newCommand = func(ctx context.Context, name string, args ...string) command {
	return &execCommand{cmd: exec.CommandContext(ctx, name, args...)}
}

func (e *execCommand) StdinPipe() (io.WriteCloser, error) {
	return e.cmd.StdinPipe()
}

func (e *execCommand) SetStdin(r io.Reader) {
	e.cmd.Stdin = r
}

func (e *execCommand) SetStdout(w io.Writer) {
	e.cmd.Stdout = w
}

func (e *execCommand) SetStderr(w io.Writer) {
	e.cmd.Stderr = w
}

func (e *execCommand) Start() error {
	return e.cmd.Start()
}

func (e *execCommand) Wait() error {
	return e.cmd.Wait()
}

func (e *execCommand) ExitCode() int {
	if e.cmd.ProcessState == nil {
		return -1
	}
	return e.cmd.ProcessState.ExitCode()
}
