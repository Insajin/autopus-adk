//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package cli

import (
	"errors"
	"os/exec"
	"syscall"
)

func configureWorkflowContextManagedRPCProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("managed OMP process command is missing")
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return nil
}

func terminateWorkflowContextManagedRPCProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func workflowContextManagedRPCProcessActive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return false
	}
	return syscall.Kill(cmd.Process.Pid, 0) == nil
}
