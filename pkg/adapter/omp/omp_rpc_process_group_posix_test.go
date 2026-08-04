//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package omp

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func configureOMPRPCProcessGroup(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return nil
}

func terminateOMPRPCProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func TestOMPRPCProcessGroup_TerminationKillsDescendant(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & echo $!; wait")
	require.NoError(t, configureOMPRPCProcessGroup(cmd))
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	cleaned := false
	t.Cleanup(func() {
		if !cleaned {
			_ = terminateOMPRPCProcessGroup(cmd)
			_ = cmd.Wait()
		}
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	descendantPID, err := strconv.Atoi(strings.TrimSpace(line))
	require.NoError(t, err)
	require.NoError(t, terminateOMPRPCProcessGroup(cmd))
	_ = cmd.Wait()
	cleaned = true
	requireOMPTestProcessGone(t, descendantPID)
}

func TestOMPRPCProcessGroup_RunTimeoutKillsDescendant(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed OMP RPC network isolation is only implemented on Darwin")
	}
	root := t.TempDir()
	scratch := filepath.Join(root, "scratch")
	profile := filepath.Join(root, "profile")
	require.NoError(t, os.MkdirAll(scratch, 0o700))
	require.NoError(t, os.MkdirAll(profile, 0o700))
	overlay := filepath.Join(profile, "live-config.yml")
	require.NoError(t, os.WriteFile(overlay, []byte("skills: {}\n"), 0o600))

	pidFile := filepath.Join(root, "descendant.pid")
	executable := filepath.Join(root, "fake-omp")
	script := "#!/bin/sh\nsleep 30 &\necho $! > '" + strings.ReplaceAll(pidFile, "'", "'\"'\"'") + "'\nwait\n"
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, runErr := runActualOMPRPC(ctx, executable, scratch, profile, overlay, "fixture", ompClosedProxy)
		done <- runErr
	}()

	var (
		data    []byte
		readErr error
	)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr = os.ReadFile(pidFile)
		if readErr == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if readErr != nil {
		cancel()
		<-done
		require.NoError(t, readErr)
	}
	cancel()
	require.ErrorContains(t, <-done, "rpc_timeout")

	descendantPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	descendantGone := false
	t.Cleanup(func() {
		if !descendantGone {
			_ = syscall.Kill(descendantPID, syscall.SIGKILL)
		}
	})
	requireOMPTestProcessGone(t, descendantPID)
	descendantGone = true
}

func requireOMPTestProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH)
}
