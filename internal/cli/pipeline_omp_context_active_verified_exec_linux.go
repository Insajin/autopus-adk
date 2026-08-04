//go:build linux

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: Linux build-selected verified-command state shared by constructors, sandbox validation, and startup.
// @AX:REASON [AUTO]: common execution code depends on this type and its FD-backed identity fields remaining compatible across platform implementations.
type pipelineOMPVerifiedExecCommand struct {
	cmd                    *exec.Cmd
	parentFD               *os.File
	expected               pipelineOMPExecutableIdentity
	gateContext            context.Context
	afterFirstDarwinStop   func()
	inheritedDarwinPath    bool
	inheritedDarwinPrivate bool
}

func newPipelineOMPVerifiedExecCommand(
	path string,
	expected pipelineOMPExecutableIdentity,
	args ...string,
) (*pipelineOMPVerifiedExecCommand, error) {
	return newPipelineOMPVerifiedExecCommandContext(context.Background(), path, expected, args...)
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
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: fd 3 is the first descriptor assigned from exec.Cmd.ExtraFiles to the verified executable.
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
