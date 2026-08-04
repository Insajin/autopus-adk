package cli

import (
	"context"
	"errors"
	"io"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the 4 KiB cap bounds untrusted executable version output before identity parsing.
const workflowContextObserveVersionOutputLimit = 4 << 10

type workflowContextObserveVersionOutput struct {
	body     []byte
	exceeded bool
}

func (output *workflowContextObserveVersionOutput) Write(data []byte) (int, error) {
	written := len(data)
	remaining := workflowContextObserveVersionOutputLimit - len(output.body)
	if remaining >= len(data) {
		output.body = append(output.body, data...)
		return written, nil
	}
	if remaining > 0 {
		output.body = append(output.body, data[:remaining]...)
	}
	output.exceeded = true
	return written, errors.New("observe-call OMP version output limit exceeded")
}

func newWorkflowContextObserveInheritedVersionCommand(
	ctx context.Context,
	options WorkflowContextManagedRPCOptions,
) (*pipelineOMPVerifiedExecCommand, pipelineOMPExecutableIdentity, error) {
	executable, identity, err := canonicalPipelineOMPExecutable(options.Executable)
	if err != nil {
		return nil, identity, err
	}
	command, err := newPipelineOMPVerifiedExecCommandContext(ctx, executable, identity, "--version")
	if err != nil {
		return nil, identity, err
	}
	command.cmd.Dir, command.cmd.Env = options.Workspace, options.Environment
	if err := configurePipelineOMPActiveSandbox(
		command.cmd, options.AllowedEndpoint, pipelineOMPActiveSandboxInheritedParent,
	); err != nil {
		return nil, identity, errors.Join(err, command.Close())
	}
	command.directDarwinImage = true
	if err := configureWorkflowContextManagedRPCProcessGroup(command.cmd); err != nil {
		return nil, identity, errors.Join(err, command.Close())
	}
	return command, identity, nil
}

func outputWorkflowContextObserveInheritedVersion(
	command *pipelineOMPVerifiedExecCommand,
	identity pipelineOMPExecutableIdentity,
) ([]byte, error) {
	output := &workflowContextObserveVersionOutput{}
	command.cmd.Stdout, command.cmd.Stderr = output, io.Discard
	if err := command.Start(); err != nil {
		return nil, err
	}
	waitErr := command.cmd.Wait()
	verifyErr := verifyPipelineOMPExecutable(command.cmd.Path, identity)
	if waitErr != nil || output.exceeded || verifyErr != nil {
		return nil, errors.Join(errors.New("observe-call verified OMP version probe failed"), waitErr, verifyErr)
	}
	return append([]byte(nil), output.body...), nil
}
