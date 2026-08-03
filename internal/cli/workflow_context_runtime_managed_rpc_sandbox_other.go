//go:build !darwin

package cli

import (
	"errors"
	"os/exec"
)

func configureWorkflowContextManagedRPCSandbox(_ *exec.Cmd, _ string) (bool, error) {
	return false, errors.New("managed OMP network sandbox is unsupported")
}
