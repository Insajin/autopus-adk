package orchestra

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// CompletionDetector defines the strategy for detecting provider completion.
// @AX:ANCHOR [AUTO] strategy interface — implemented by SignalDetector and ScreenPollDetector; used by waitForCompletion wrapper
type CompletionDetector interface {
	// WaitForCompletion blocks until the provider completes or context expires.
	// Returns true if completion was detected, false on timeout/cancel.
	WaitForCompletion(ctx context.Context, pi paneInfo, patterns []CompletionPattern, baseline string, round int) (bool, error)
}

// NewCompletionDetector creates the best CompletionDetector for the given terminal.
// If the terminal supports signals (SignalCapable), returns SignalDetector.
// Otherwise, returns ScreenPollDetector.
// For hook-mode support, use NewCompletionDetectorWithConfig instead.
// @AX:ANCHOR [AUTO] factory — fan-in point; callers: waitForCompletion, OrchestraConfig.CompletionDetector
func NewCompletionDetector(term terminal.Terminal) CompletionDetector {
	return NewCompletionDetectorWithConfig(term, false, nil)
}

// NewCompletionDetectorWithConfig creates the best CompletionDetector considering hook mode.
// Priority: SignalDetector > FileIPCDetector > ScreenPollDetector.
// @AX:ANCHOR [AUTO] factory with config — enables FileIPCDetector when hookMode=true and session is available
func NewCompletionDetectorWithConfig(term terminal.Terminal, hookMode bool, session *HookSession) CompletionDetector {
	if sc, ok := term.(terminal.SignalCapable); ok {
		return &SignalDetector{term: term, signal: sc}
	}
	if hookMode && session != nil {
		return &FileIPCDetector{session: session}
	}
	return &ScreenPollDetector{term: term}
}
