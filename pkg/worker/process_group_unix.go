//go:build !windows

package worker

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func prepareCommandProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func watchCommandCancellation(ctx context.Context, cmd *exec.Cmd, taskID string) func() {
	if ctx == nil || cmd == nil {
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() {
			close(done)
		})
	}

	go func() {
		select {
		case <-ctx.Done():
			terminateProcessGroup(cmd, taskID)
		case <-done:
		}
	}()

	return stop
}

func terminateProcessGroup(cmd *exec.Cmd, taskID string) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	pgid := -cmd.Process.Pid
	if err := syscall.Kill(pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		log.Printf("[worker] task %s: SIGTERM process group failed: %v", taskID, err)
		return
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	<-timer.C

	if err := syscall.Kill(pgid, 0); err != nil {
		return
	}
	if err := syscall.Kill(pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		log.Printf("[worker] task %s: SIGKILL process group failed: %v", taskID, err)
	}
}
