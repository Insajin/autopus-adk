//go:build !darwin && !linux

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

type pipelineOMPVerifiedExecCommand struct {
	cmd                    *exec.Cmd
	parentFD               *os.File
	inheritedDarwinPath    bool
	inheritedDarwinPrivate bool
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: build-selected sandbox-mode contract shared by process startup, version probes, and platform tests.
// @AX:REASON [AUTO]: unsupported platforms must preserve the common signature while rejecting inherited execution fail closed.
func configurePipelineOMPVerifiedExecSandboxMode(
	*pipelineOMPVerifiedExecCommand,
	pipelineOMPActiveSandboxMode,
	bool,
) error {
	return errors.New("verified managed active OMP execution is unsupported")
}

func pipelineOMPVerifiedExecUsesDarwinPtrace(*pipelineOMPVerifiedExecCommand) bool { return false }

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
