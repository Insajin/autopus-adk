//go:build !darwin

package cli

import "os/exec"

func workflowContextLiveSandboxRequired() bool { return false }

func configureWorkflowContextLiveSandbox(_ *exec.Cmd, _ string) (bool, error) {
	return false, nil
}
