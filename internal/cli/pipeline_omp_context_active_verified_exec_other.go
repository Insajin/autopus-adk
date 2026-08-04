//go:build !darwin && !linux

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

type pipelineOMPVerifiedExecCommand struct {
	cmd               *exec.Cmd
	parentFD          *os.File
	directDarwinImage bool
}

func newPipelineOMPVerifiedExecCommand(
	string,
	pipelineOMPExecutableIdentity,
	...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	return nil, errors.New("verified managed active OMP execution is unsupported")
}

func newPipelineOMPVerifiedExecCommandWithGate(
	context.Context,
	string,
	pipelineOMPExecutableIdentity,
	...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	return nil, errors.New("verified managed active OMP execution is unsupported")
}

func newPipelineOMPVerifiedExecCommandContext(
	context.Context,
	string,
	pipelineOMPExecutableIdentity,
	...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	return nil, errors.New("verified managed active OMP execution is unsupported")
}

func (command *pipelineOMPVerifiedExecCommand) Close() error { return nil }

func (command *pipelineOMPVerifiedExecCommand) Start() error {
	return errors.New("verified managed active OMP execution is unsupported")
}
