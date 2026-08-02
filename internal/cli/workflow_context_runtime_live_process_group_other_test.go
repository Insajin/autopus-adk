//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !solaris

package cli

import (
	"errors"
	"os/exec"
)

func configureWorkflowContextLiveProcessGroup(_ *exec.Cmd) error {
	return errors.New("process-groups-unsupported")
}

func captureWorkflowContextLiveProcessGroup(_ *exec.Cmd) (int, error) {
	return 0, errors.New("process-groups-unsupported")
}

func terminateWorkflowContextLiveProcessGroup(_ int) error {
	return errors.New("process-groups-unsupported")
}
