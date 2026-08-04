//go:build darwin

package cli

import (
	"context"
	"os"
	"os/exec"
)

type pipelineOMPVerifiedExecCommand struct {
	cmd                       *exec.Cmd
	parentFD                  *os.File
	expected                  pipelineOMPExecutableIdentity
	gateContext               context.Context
	afterFirstDarwinStop      func()
	afterInheritedDarwinStart func()
	inheritedDarwinPath       bool
	inheritedDarwinPrivate    bool
}
