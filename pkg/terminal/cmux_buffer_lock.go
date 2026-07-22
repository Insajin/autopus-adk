package terminal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	cmuxInputBufferName         = "autopus-input"
	cmuxInputBufferSentinel     = "[autopus-cleared]"
	cmuxBufferPasteSettleDelay  = 250 * time.Millisecond
	cmuxBufferClearTimeout      = 750 * time.Millisecond
	cmuxBufferPostPasteMaxDelay = cmuxBufferPasteSettleDelay + cmuxBufferClearTimeout
)

var (
	errCmuxBufferLockBusy   = errors.New("cmux input buffer lock busy")
	cmuxBufferProcessGate   = make(chan struct{}, 1)
	cmuxBufferLockPath      = filepath.Join(os.TempDir(), "autopus-cmux-input.flock")
	cmuxBufferLockWaitLimit = 2 * time.Second
)

// acquireCmuxInputBuffer serializes the fixed cmux buffer. The advisory file
// lock is released by the OS when an owner exits, including crashes.
func acquireCmuxInputBuffer(ctx context.Context) (func(), error) {
	lockCtx, cancel := context.WithTimeout(ctx, cmuxBufferLockWaitLimit)
	defer cancel()
	select {
	case cmuxBufferProcessGate <- struct{}{}:
	case <-lockCtx.Done():
		return nil, cmuxBufferWaitError(ctx)
	}
	file, err := openCmuxBufferLockFile(cmuxBufferLockPath)
	if err != nil {
		<-cmuxBufferProcessGate
		return nil, err
	}
	for {
		busy, lockErr := tryLockCmuxBufferFile(file)
		if lockErr != nil {
			_ = file.Close()
			<-cmuxBufferProcessGate
			return nil, lockErr
		}
		if !busy {
			if err := validateCmuxBufferLockFile(file, cmuxBufferLockPath); err != nil {
				_ = unlockCmuxBufferFile(file)
				_ = file.Close()
				<-cmuxBufferProcessGate
				return nil, err
			}
			var once sync.Once
			return func() {
				once.Do(func() {
					if err := unlockCmuxBufferFile(file); err != nil {
						log.Printf("cmux: unlock input buffer: %v", err)
					}
					_ = file.Close()
					<-cmuxBufferProcessGate
				})
			}, nil
		}
		select {
		case <-lockCtx.Done():
			_ = file.Close()
			<-cmuxBufferProcessGate
			return nil, cmuxBufferWaitError(ctx)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func openCmuxBufferLockFile(lockPath string) (*os.File, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open cmux input buffer lock: %w", err)
	}
	if err := validateCmuxBufferLockFile(file, lockPath); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func validateCmuxBufferLockFile(file *os.File, lockPath string) error {
	descriptorInfo, descriptorErr := file.Stat()
	pathInfo, pathErr := os.Lstat(lockPath)
	if descriptorErr != nil || pathErr != nil || !descriptorInfo.Mode().IsRegular() ||
		!pathInfo.Mode().IsRegular() || descriptorInfo.Mode().Perm() != 0o600 ||
		pathInfo.Mode().Perm() != 0o600 || !os.SameFile(descriptorInfo, pathInfo) ||
		!cmuxBufferLockOwnedByCurrentUser(descriptorInfo) {
		return errors.New("invalid cmux input buffer lock")
	}
	return nil
}

func cmuxBufferWaitError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w after %s", errCmuxBufferLockBusy, cmuxBufferLockWaitLimit)
}

// settleAndClearCmuxInputBuffer lets cmux's async paste queue consume the
// payload before overwrite. Cleanup survives caller cancellation, but the
// cancellation is returned after at most cmuxBufferPostPasteMaxDelay.
func settleAndClearCmuxInputBuffer(callerCtx context.Context) error {
	timer := time.NewTimer(cmuxBufferPasteSettleDelay)
	defer timer.Stop()
	var callerErr error
	select {
	case <-timer.C:
	case <-callerCtx.Done():
		callerErr = callerCtx.Err()
		<-timer.C
	}
	clearCmuxInputBuffer()
	if callerErr == nil {
		callerErr = callerCtx.Err()
	}
	return callerErr
}

// clearCmuxInputBuffer uses an independent bounded context. A later send
// overwrites the same fixed buffer even when this diagnostic cleanup fails.
func clearCmuxInputBuffer() {
	ctx, cancel := context.WithTimeout(context.Background(), cmuxBufferClearTimeout)
	defer cancel()
	cmd := execCommandContext(
		ctx, "cmux", "set-buffer", "--name", cmuxInputBufferName, "--", cmuxInputBufferSentinel,
	)
	if err := cmd.Run(); err != nil {
		log.Printf("cmux: clear input buffer: %v", err)
	}
}
