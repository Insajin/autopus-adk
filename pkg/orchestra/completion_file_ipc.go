package orchestra

import (
	"context"
	"fmt"
	"time"
)

// FileIPCDetector uses file-based IPC signals (via HookSession) for completion detection.
// SECONDARY detector: used when the terminal lacks signal support but HookMode is enabled.
// Faster than ScreenPollDetector (200ms file polling vs 2s screen polling).
// @AX:ANCHOR [AUTO] file-IPC completion strategy — bridges HookSession to CompletionDetector interface
type FileIPCDetector struct {
	session *HookSession
}

// defaultFileIPCTimeout is the fallback timeout when the context has no deadline.
const defaultFileIPCTimeout = 10 * time.Minute

// WaitForCompletion polls for the provider's done signal file via HookSession.
// Uses round-scoped signals when round > 0, otherwise uses the standard done file.
// Timeout is derived from the context deadline; falls back to defaultFileIPCTimeout.
func (d *FileIPCDetector) WaitForCompletion(ctx context.Context, pi paneInfo, _ []CompletionPattern, _ string, round int) (bool, error) {
	provider := pi.provider.Name
	if provider == "" {
		return false, fmt.Errorf("FileIPCDetector: provider name is empty")
	}

	timeout := fileIPCTimeout(ctx)

	var err error
	if round > 0 {
		err = d.session.WaitForDoneRoundCtx(ctx, timeout, provider, round)
	} else {
		err = d.session.WaitForDone(timeout, provider)
	}

	if err != nil {
		// Context cancellation means the caller gave up -- not an error.
		if ctx.Err() != nil {
			return false, nil
		}
		// Timeout from HookSession -- treat as "not completed yet".
		return false, nil
	}

	return true, nil
}

// fileIPCTimeout extracts the remaining time from the context deadline.
// Returns defaultFileIPCTimeout if no deadline is set.
func fileIPCTimeout(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return defaultFileIPCTimeout
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		remaining = 1 * time.Second
	}
	return remaining
}
