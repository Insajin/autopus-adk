//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !solaris

package cli

import (
	"errors"
	"os/exec"
)

func configureWorkflowContextManagedRPCProcessGroup(_ *exec.Cmd) error {
	return errors.New("managed OMP process groups are unsupported")
}

func terminateWorkflowContextManagedRPCProcessGroup(_ *exec.Cmd) error { return nil }

func workflowContextManagedRPCProcessActive(_ *exec.Cmd) bool { return false }
