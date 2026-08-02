//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package cli

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func configureWorkflowContextLiveProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("process-group-command-missing")
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return nil
}

func captureWorkflowContextLiveProcessGroup(cmd *exec.Cmd) (int, error) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return 0, errors.New("process-group-leader-missing")
	}
	groupID, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || groupID != cmd.Process.Pid {
		return 0, errors.New("process-group-identity-unverified")
	}
	return groupID, nil
}

func terminateWorkflowContextLiveProcessGroup(groupID int) error {
	if groupID <= 0 {
		return errors.New("process-group-id-invalid")
	}
	err := syscall.Kill(-groupID, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func TestWorkflowContextLiveProcessTree_TerminalPathsKillDescendantsWithoutCollateral(t *testing.T) {
	sentinel := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := sentinel.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sentinel.Process.Kill()
		_ = sentinel.Wait()
	})

	for _, terminal := range []string{"success", "error", "timeout"} {
		t.Run(terminal, func(t *testing.T) {
			descendantPID, terminalErr := runWorkflowContextLiveProcessTreeFixture(terminal)
			switch terminal {
			case "success":
				if terminalErr != nil {
					t.Fatal(terminalErr)
				}
			case "error":
				if !errors.Is(terminalErr, errWorkflowContextLiveProcessTreeFixture) {
					t.Fatalf("unexpected terminal error: %v", terminalErr)
				}
			case "timeout":
				if !errors.Is(terminalErr, context.DeadlineExceeded) {
					t.Fatalf("unexpected timeout error: %v", terminalErr)
				}
			}
			requireWorkflowContextLiveProcessGone(t, descendantPID)
			if err := syscall.Kill(sentinel.Process.Pid, 0); err != nil {
				t.Fatalf("unrelated process was affected by group cleanup: %v", err)
			}
		})
	}
}

var errWorkflowContextLiveProcessTreeFixture = errors.New("fixture-error")

func runWorkflowContextLiveProcessTreeFixture(terminal string) (descendantPID int, terminalErr error) {
	ctx := context.Background()
	cancel := func() {}
	if terminal == "timeout" {
		ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 30 & echo $!; wait")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	tree := newWorkflowContextLiveProcessTree(cmd)
	if err := tree.Start(); err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := tree.Close(); terminalErr == nil && closeErr != nil {
			terminalErr = closeErr
		}
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		return 0, err
	}
	descendantPID, err = strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return 0, err
	}
	switch terminal {
	case "success":
		return descendantPID, nil
	case "error":
		return descendantPID, errWorkflowContextLiveProcessTreeFixture
	case "timeout":
		<-ctx.Done()
		return descendantPID, ctx.Err()
	default:
		return descendantPID, errors.New("unknown terminal")
	}
}

func requireWorkflowContextLiveProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("descendant process still exists: pid=%d err=%v", pid, err)
	}
}
