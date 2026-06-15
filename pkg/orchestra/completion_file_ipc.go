package orchestra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// WaitForCompletion polls for either the provider's response-file marker or its
// done signal file via HookSession. Uses round-scoped signals when round > 0,
// otherwise uses the standard done file. Timeout is derived from the context
// deadline; falls back to defaultFileIPCTimeout.
func (d *FileIPCDetector) WaitForCompletion(ctx context.Context, pi paneInfo, _ []CompletionPattern, _ string, round int) (bool, error) {
	provider := pi.provider.Name
	if provider == "" {
		return false, fmt.Errorf("FileIPCDetector: provider name is empty")
	}

	timeout := fileIPCTimeout(ctx)
	doneName := sanitizeProviderName(provider) + "-done"
	if round > 0 {
		doneName = RoundSignalName(provider, round, "done")
	}
	return d.waitForDoneOrResponseFile(ctx, timeout, doneName, pi.responseFile)
}

func (d *FileIPCDetector) waitForDoneOrResponseFile(ctx context.Context, timeout time.Duration, doneName, responseFile string) (bool, error) {
	if _, ok := readResponseFile(responseFile); ok {
		return true, nil
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	donePath := filepath.Join(d.session.Dir(), doneName)
	if _, err := os.Stat(donePath); err == nil {
		return true, nil
	}
	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-deadline.C:
			return false, nil
		case <-ticker.C:
			if _, ok := readResponseFile(responseFile); ok {
				return true, nil
			}
			if _, err := os.Stat(donePath); err == nil {
				return true, nil
			}
		}
	}
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
