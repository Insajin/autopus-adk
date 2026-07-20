//go:build !windows

package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/require"
)

func TestExecuteSubprocess_ContextCancelKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := fmt.Sprintf("sleep 30 & echo $! > %q; wait", pidFile)
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, err := wl.executeSubprocess(ctx, adapter.TaskConfig{
		TaskID: "ctx-kill-group",
		Prompt: "do work",
	})
	require.Error(t, err)

	var childPID int
	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(pidFile)
		if readErr != nil {
			return false
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if convErr != nil {
			return false
		}
		childPID = pid
		return childPID > 0
	}, 2*time.Second, 50*time.Millisecond)

	require.Eventually(t, func() bool {
		return !processRunning(childPID)
	}, 6*time.Second, 100*time.Millisecond)
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err != nil && err != syscall.EPERM {
		return false
	}

	// A stopped child can remain as a zombie when the test binary is PID 1 in
	// a container. kill(pid, 0) still succeeds for zombies, but they are no
	// longer running and therefore satisfy the process-termination contract.
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	stat, statErr := os.ReadFile(statPath)
	if statErr == nil {
		stateOffset := strings.LastIndex(string(stat), ") ")
		if stateOffset >= 0 && len(stat) > stateOffset+2 {
			state := stat[stateOffset+2]
			return state != 'Z' && state != 'X'
		}
		return true
	}
	if _, procErr := os.Stat("/proc/self/stat"); procErr == nil && os.IsNotExist(statErr) {
		return false
	}
	return true
}
