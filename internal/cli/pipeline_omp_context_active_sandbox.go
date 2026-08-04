package cli

import (
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: shared sandbox boundary for managed child startup and version probes.
// @AX:REASON [AUTO]: active execution, observe-call, and inherited-version callers depend on the mode switch remaining fail closed and Darwin-only.
func configurePipelineOMPActiveSandbox(
	cmd *exec.Cmd,
	endpoint string,
	mode pipelineOMPActiveSandboxMode,
) error {
	if cmd == nil || cmd.Path == "" {
		return errors.New("managed active OMP sandbox command is unavailable")
	}
	switch mode {
	case pipelineOMPActiveSandboxManaged:
		_, err := configureWorkflowContextManagedRPCSandbox(cmd, endpoint)
		return err
	case pipelineOMPActiveSandboxInheritedParent:
		if runtime.GOOS != "darwin" {
			return errors.New("inherited parent sandbox is supported only on Darwin")
		}
		if _, err := validatePipelineOMPActiveEndpoint(endpoint); err != nil {
			return errors.New("inherited parent sandbox endpoint is invalid")
		}
		return nil
	default:
		return errors.New("managed active OMP sandbox mode is invalid")
	}
}

func pipelineOMPMaxTimeSeconds(maxTime time.Duration) string {
	seconds := maxTime / time.Second
	if maxTime%time.Second > 0 {
		seconds++
	}
	return strconv.FormatInt(int64(seconds), 10)
}
