package orchestra

import (
	"context"
	"fmt"
	"log"
	"time"
)

// fileIPCReadyTimeout is the default timeout for waiting for a provider's ready signal.
// @AX:NOTE [AUTO] magic constant 30s — must be less than per-round timeout; increase if providers are slow to signal ready
const fileIPCReadyTimeout = 30 * time.Second

type fileIPCOutcome uint8

const (
	fileIPCDelivered fileIPCOutcome = iota
	fileIPCSafeFallback
	fileIPCReleaseFailure
)

// tryFileIPC attempts to deliver a prompt via file IPC for hook-capable providers.
// Safe fallback is returned only after the hook acknowledges abort by removing
// both ready and abort artifacts. Release failures must never reach pane input.
func tryFileIPC(ctx context.Context, hookSession *HookSession, provider string, round int, prompt string) (fileIPCOutcome, error) {
	return tryFileIPCWithTimeouts(
		ctx, hookSession, provider, round, prompt, fileIPCReadyTimeout, defaultHookReleaseTimeout,
	)
}

func tryFileIPCWithTimeouts(
	ctx context.Context,
	hookSession *HookSession,
	provider string,
	round int,
	prompt string,
	readyTimeout time.Duration,
	releaseTimeout time.Duration,
) (fileIPCOutcome, error) {
	if err := hookSession.WaitForReadyCtx(ctx, readyTimeout, provider, round); err != nil {
		return releaseFileIPCFallback(
			ctx, hookSession, provider, round, fmt.Errorf("wait for ready: %w", err), releaseTimeout,
		)
	}

	if err := hookSession.WriteInputRound(provider, round, prompt); err != nil {
		return releaseFileIPCFallback(
			ctx, hookSession, provider, round, fmt.Errorf("write input: %w", err), releaseTimeout,
		)
	}

	return fileIPCDelivered, nil
}

func releaseFileIPCFallback(
	ctx context.Context,
	hookSession *HookSession,
	provider string,
	round int,
	cause error,
	timeout time.Duration,
) (fileIPCOutcome, error) {
	if err := hookSession.WriteAbortSignal(provider, round); err != nil {
		releaseErr := fmt.Errorf("file IPC failed (%v); write abort: %w", cause, err)
		log.Printf("[Round %d] %s release failed: %v", round, provider, releaseErr)
		return fileIPCReleaseFailure, releaseErr
	}
	targets := []hookReleaseTarget{newHookReleaseTarget(provider, round)}
	if err := waitForHookReleaseAcknowledgement(ctx, hookSession, targets, round, timeout); err != nil {
		releaseErr := fmt.Errorf("file IPC failed (%v); acknowledge hook release: %w", cause, err)
		log.Printf("[Round %d] %s release failed: %v", round, provider, releaseErr)
		return fileIPCReleaseFailure, releaseErr
	}
	log.Printf("[Round %d] %s %v — falling back to direct pane input", round, provider, cause)
	return fileIPCSafeFallback, cause
}
