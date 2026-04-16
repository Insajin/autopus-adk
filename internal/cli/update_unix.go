//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

// isWritable checks if the directory is writable by the current user.
func isWritable(dir string) bool {
	return syscall.Access(dir, syscall.O_RDWR) == nil
}

// reExecWithSudo re-executes the current command with sudo, inheriting stdin/stdout/stderr.
func reExecWithSudo() error {
	pathInfo, err := resolveCurrentBinaryPath()
	if err != nil {
		return err
	}
	args := append([]string{pathInfo.ManagedPath()}, os.Args[1:]...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
