//go:build windows

package worker

import (
	"context"
	"os/exec"
)

func prepareCommandProcessGroup(cmd *exec.Cmd) {}

func watchCommandCancellation(ctx context.Context, cmd *exec.Cmd, taskID string) func() {
	return func() {}
}
