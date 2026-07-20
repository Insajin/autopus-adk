//go:build windows

package processprobe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

const (
	windowsProbeModeEnv   = "AUTOPUS_WINDOWS_PROBE_MODE"
	windowsProbePIDEnv    = "AUTOPUS_WINDOWS_PROBE_PID_FILE"
	windowsProbeMarkerEnv = "AUTOPUS_WINDOWS_PROBE_MARKER_FILE"
)

func TestOutputWindowsFastPath(t *testing.T) {
	cmd, cancel := windowsProbeCommand(t, "version", 2*time.Second)
	defer cancel()

	out, err := Output(cmd)

	require.NoError(t, err)
	assert.Equal(t, "1.2.3", strings.TrimSpace(string(out)))
}

func TestOutputWindowsExitFailure(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd, cancel := windowsProbeCommand(t, "failure", 2*time.Second)
	defer cancel()
	cmd.Env = append(cmd.Env, windowsProbePIDEnv+"="+pidFile)

	_, err := Output(cmd)

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Contains(t, string(exitErr.Stderr), "probe stderr marker")
	childPID, readErr := readWindowsProbePID(pidFile)
	require.NoError(t, readErr)
	requireWindowsProcessExit(t, childPID, 2*time.Second)
}

func TestOutputWindowsHungProcessReturnsWithinBound(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd, cancel := windowsProbeCommand(t, "hang", time.Second)
	defer cancel()
	cmd.Env = append(cmd.Env, windowsProbePIDEnv+"="+pidFile)
	started := time.Now()

	_, err := Output(cmd)

	assert.Error(t, err)
	assert.Less(t, time.Since(started), 4*time.Second)
	childPID, readErr := readWindowsProbePID(pidFile)
	require.NoError(t, readErr)
	requireWindowsProcessExit(t, childPID, 2*time.Second)
}

func TestOutputWindowsGrandchildPipeReturnsWithinBound(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd, cancel := windowsProbeCommand(t, "pipe-parent", 2*time.Second)
	defer cancel()
	cmd.Env = append(cmd.Env, windowsProbePIDEnv+"="+pidFile)
	started := time.Now()

	_, err := Output(cmd)

	assert.ErrorIs(t, err, exec.ErrWaitDelay)
	assert.Less(t, time.Since(started), 2*time.Second)
	childPID, readErr := readWindowsProbePID(pidFile)
	require.NoError(t, readErr)
	requireWindowsProcessExit(t, childPID, 2*time.Second)
}

func TestOutputWindowsSuccessKeepsDetachedChildAlive(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "child.pid")
	markerFile := filepath.Join(tempDir, "completed")
	cmd, cancel := windowsProbeCommand(t, "success-parent", 2*time.Second)
	defer cancel()
	cmd.Env = append(cmd.Env,
		windowsProbePIDEnv+"="+pidFile,
		windowsProbeMarkerEnv+"="+markerFile,
	)

	out, err := Output(cmd)

	require.NoError(t, err)
	assert.Equal(t, "1.2.3", strings.TrimSpace(string(out)))
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(markerFile)
		return statErr == nil
	}, 2*time.Second, 20*time.Millisecond)
	childPID, readErr := readWindowsProbePID(pidFile)
	require.NoError(t, readErr)
	requireWindowsProcessExit(t, childPID, 2*time.Second)
}

func TestOutputWindowsHelperProcess(t *testing.T) {
	mode := os.Getenv(windowsProbeModeEnv)
	if mode == "" {
		return
	}
	switch mode {
	case "version":
		fmt.Fprintln(os.Stdout, "1.2.3")
	case "failure":
		startWindowsProbeBackgroundChild(t)
		fmt.Fprintln(os.Stderr, "probe stderr marker")
		os.Exit(17)
	case "hang":
		startWindowsProbeBackgroundChild(t)
		time.Sleep(5 * time.Second)
	case "background-child":
		time.Sleep(30 * time.Second)
	case "pipe-child":
		time.Sleep(5 * time.Second)
	case "success-child":
		time.Sleep(200 * time.Millisecond)
		require.NoError(t, os.WriteFile(os.Getenv(windowsProbeMarkerEnv), []byte("done"), 0o600))
	case "pipe-parent":
		path, err := os.Executable()
		require.NoError(t, err)
		cmd := exec.Command(path, windowsProbeHelperArg())
		cmd.Env = append(os.Environ(), windowsProbeModeEnv+"=pipe-child")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		require.NoError(t, cmd.Start())
		require.NoError(t, os.WriteFile(os.Getenv(windowsProbePIDEnv),
			[]byte(strconv.Itoa(cmd.Process.Pid)), 0o600))
	case "success-parent":
		path, err := os.Executable()
		require.NoError(t, err)
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		require.NoError(t, err)
		defer devNull.Close()
		cmd := exec.Command(path, windowsProbeHelperArg())
		cmd.Env = append(os.Environ(),
			windowsProbeModeEnv+"=success-child",
			windowsProbeMarkerEnv+"="+os.Getenv(windowsProbeMarkerEnv),
		)
		cmd.Stdout = devNull
		cmd.Stderr = devNull
		require.NoError(t, cmd.Start())
		require.NoError(t, os.WriteFile(os.Getenv(windowsProbePIDEnv),
			[]byte(strconv.Itoa(cmd.Process.Pid)), 0o600))
		fmt.Fprintln(os.Stdout, "1.2.3")
	}
	// Keep fixture stdout free of the test runner's PASS banner.
	os.Exit(0)
}

func windowsProbeCommand(t *testing.T, mode string, timeout time.Duration) (*exec.Cmd, context.CancelFunc) {
	t.Helper()
	path, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, path, windowsProbeHelperArg())
	cmd.Env = append(os.Environ(), windowsProbeModeEnv+"="+mode)
	return cmd, cancel
}

func windowsProbeHelperArg() string {
	return "-test.run=^TestOutputWindowsHelperProcess$"
}

func startWindowsProbeBackgroundChild(t *testing.T) {
	t.Helper()
	path, err := os.Executable()
	require.NoError(t, err)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	cmd := exec.Command(path, windowsProbeHelperArg())
	cmd.Env = append(os.Environ(), windowsProbeModeEnv+"=background-child")
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	require.NoError(t, cmd.Start())
	require.NoError(t, devNull.Close())
	require.NoError(t, os.WriteFile(os.Getenv(windowsProbePIDEnv),
		[]byte(strconv.Itoa(cmd.Process.Pid)), 0o600))
}

func readWindowsProbePID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func requireWindowsProcessExit(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return
	}
	require.NoError(t, err)
	defer windows.CloseHandle(handle)

	event, err := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_OBJECT_0), event, "probe descendant did not exit")
}
