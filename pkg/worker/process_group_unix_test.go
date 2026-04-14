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
		return !processAlive(childPID)
	}, 6*time.Second, 100*time.Millisecond)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
