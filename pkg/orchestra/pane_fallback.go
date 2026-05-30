package orchestra

import (
	"context"
	"fmt"
)

// noneBackendMarker records that neither the pane nor the subprocess backend
// produced a usable response (REQ-013).
const noneBackendMarker = "none"

// recoveryHint is the concrete operator-facing recovery instruction surfaced
// when both backends are unavailable (REQ-013/S14).
const recoveryHint = "ensure a logged-in cmux/tmux CLI session and that the provider CLI is logged in"

// paneFallback handles best-effort subprocess fallback after interactive-pane
// execution fails BEFORE producing a deterministic result (REQ-005/013).
//
// On subprocess success it returns the subprocess response tagged
// ExecutedBackend="subprocess" (recoverable path, S6). When the subprocess
// backend is also unavailable it returns an actionable error naming BOTH
// failure causes plus a recovery instruction, and a response marked so neither
// backend is recorded as successful (S14).
func paneFallback(ctx context.Context, req ProviderRequest, paneFailureReason string) (*ProviderResponse, error) {
	resp, err := NewSubprocessBackendImpl().Execute(ctx, req)
	if err == nil && resp != nil && !subprocessFailed(resp) {
		resp.ExecutedBackend = "subprocess"
		return resp, nil
	}

	// Both backends failed: build an actionable error (REQ-013/S14) rather than
	// surfacing a raw provider/API error.
	subReason := "the -p subprocess fallback was unavailable"
	if err != nil {
		subReason = fmt.Sprintf("the -p subprocess fallback was unavailable (%v)", err)
	} else if resp != nil && subprocessFailed(resp) {
		subReason = "the -p subprocess fallback produced no usable output"
	}

	actionable := fmt.Errorf("%s AND %s; recovery: %s",
		paneFailureReason, subReason, recoveryHint)

	failed := &ProviderResponse{
		Provider:        req.Provider,
		Error:           actionable.Error(),
		ExecutedBackend: noneBackendMarker,
	}
	return failed, actionable
}

// subprocessFailed reports whether a subprocess response should be treated as a
// failure (timed out or empty), so we do not pass off an empty fallback as success.
func subprocessFailed(resp *ProviderResponse) bool {
	return resp.TimedOut || resp.EmptyOutput
}
