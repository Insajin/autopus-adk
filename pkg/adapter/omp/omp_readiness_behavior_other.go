//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !solaris

package omp

import (
	"context"
	"errors"
	"os/exec"
)

func runOMPReadinessRPCCommand(
	context.Context,
	*exec.Cmd,
	[]byte,
	int,
) ([]byte, error) {
	return nil, errors.New("OMP readiness RPC process groups are unavailable")
}
