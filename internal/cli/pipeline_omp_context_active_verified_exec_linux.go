//go:build linux

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

func newPipelineOMPVerifiedExecCommand(
	path string,
	expected pipelineOMPExecutableIdentity,
	args ...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	return newPipelineOMPVerifiedExecCommandContext(nil, path, expected, args...)
}

func newPipelineOMPVerifiedExecCommandWithGate(
	ctx context.Context,
	path string,
	expected pipelineOMPExecutableIdentity,
	args ...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	command, err := newPipelineOMPVerifiedExecCommand(path, expected, args...)
	if command != nil {
		command.gateContext = ctx
	}
	return command, err
}

func newPipelineOMPVerifiedExecCommandContext(
	ctx context.Context,
	path string,
	expected pipelineOMPExecutableIdentity,
	args ...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	file, err := openPipelineOMPVerifiedExecutable(path, expected)
	if err != nil {
		return nil, err
	}
	const childPath = "/proc/self/fd/3"
	var cmd *exec.Cmd
	if ctx == nil {
		cmd = exec.Command(childPath, args...)
	} else {
		cmd = exec.CommandContext(ctx, childPath, args...)
	}
	cmd.ExtraFiles = []*os.File{file}
	return &pipelineOMPVerifiedExecCommand{cmd: cmd, parentFD: file, expected: expected, gateContext: ctx}, nil
}

func (command *pipelineOMPVerifiedExecCommand) Start() error {
	if command == nil || command.cmd == nil || command.parentFD == nil {
		return errors.New("verified managed active OMP command is unavailable")
	}
	startErr := command.cmd.Start()
	closeErr := command.Close()
	if startErr != nil {
		return errors.Join(startErr, closeErr)
	}
	if closeErr == nil {
		return nil
	}
	killErr := terminateWorkflowContextManagedRPCProcessGroup(command.cmd)
	waitErr := command.cmd.Wait()
	return errors.Join(errors.New("close verified managed active OMP parent FD"), closeErr, killErr, waitErr)
}
