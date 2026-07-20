// Package processprobe runs short-lived subprocess probes without unbounded pipe waits.
package processprobe

import (
	"os/exec"
	"time"
)

const defaultWaitDelay = 250 * time.Millisecond

var outputCommand = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

// Output captures cmd output and bounds inherited-pipe draining after process exit.
// Callers must construct cmd with a bounded context when the process itself may hang.
func Output(cmd *exec.Cmd) ([]byte, error) {
	configureProcessGroup(cmd)
	if cmd.WaitDelay <= 0 {
		cmd.WaitDelay = defaultWaitDelay
	}
	out, err := outputCommand(cmd)
	if err != nil {
		terminateProcessGroup(cmd)
	}
	return out, err
}
