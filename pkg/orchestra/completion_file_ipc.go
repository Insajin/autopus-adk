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

// codexResponseFileGrace gives a trusted Stop hook time to publish result+done
// before a valid marked response file becomes the compatibility completion
// signal. Project-local Codex hooks are skipped until the user trusts their
// exact definition, so done-only waiting would otherwise consume the full
// provider budget immediately after install or update.
const codexResponseFileGrace = time.Second

// WaitForCompletion polls HookSession artifacts. Codex gives its done signal
// priority so a Stop hook can finish before pane cleanup. A complete marked
// response becomes a compatibility signal only after a short grace period,
// preventing project hook trust from turning into a full-budget timeout. Other
// providers retain the immediate response-file path. Timeout is derived from the
// context deadline and falls back to defaultFileIPCTimeout.
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
	isCodex := isCodexInteractiveProvider(pi.provider)
	responseGrace := time.Duration(0)
	if isCodex {
		responseGrace = codexResponseFileGrace
	}
	return d.waitForCompletionSignal(ctx, timeout, doneName, pi.responseFile, !isCodex, responseGrace)
}

func (d *FileIPCDetector) waitForCompletionSignal(
	ctx context.Context,
	timeout time.Duration,
	doneName string,
	responseFile string,
	allowResponseFile bool,
	responseGrace time.Duration,
) (bool, error) {
	if allowResponseFile {
		if _, ok := readResponseFile(responseFile); ok {
			return true, nil
		}
	}
	var responseObservedAt time.Time
	if responseGrace > 0 {
		if _, ok := readResponseFile(responseFile); ok {
			responseObservedAt = time.Now()
		}
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
			if _, err := os.Stat(donePath); err == nil {
				return true, nil
			}
			if allowResponseFile {
				if _, ok := readResponseFile(responseFile); ok {
					return true, nil
				}
			} else if responseGrace > 0 {
				if _, ok := readResponseFile(responseFile); !ok {
					responseObservedAt = time.Time{}
				} else if responseObservedAt.IsZero() {
					responseObservedAt = time.Now()
				} else if time.Since(responseObservedAt) >= responseGrace {
					return true, nil
				}
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
