package orchestra

import (
	"context"
	"errors"
	"fmt"
)

// noneBackendMarker records that neither the pane nor the subprocess backend
// produced a usable response (REQ-013).
const noneBackendMarker = "none"

// recoveryHint is the concrete operator-facing recovery instruction surfaced
// when both backends are unavailable (REQ-013/S14).
const recoveryHint = "ensure a logged-in cmux/tmux CLI session and that the provider CLI is logged in"

// paneProvisioningFallback handles best-effort subprocess fallback only when
// no pane was committed (no terminal or SplitPane failed). Once SplitPane
// returns a non-empty pane ID, execution failures must stay on the pane path.
//
// On subprocess success it returns the subprocess response tagged
// ExecutedBackend="subprocess" (recoverable path, S6). When the subprocess
// backend is also unavailable it returns an actionable error naming BOTH
// failure causes plus a recovery instruction, and a response marked so neither
// backend is recorded as successful (S14).
func paneProvisioningFallback(ctx context.Context, req ProviderRequest, paneFailureReason string) (*ProviderResponse, error) {
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

// paneExecutionFailure reports a failure after pane provisioning committed.
// It deliberately does not invoke the subprocess backend.
func paneExecutionFailure(req ProviderRequest, reason string) (*ProviderResponse, error) {
	err := errors.New(reason)
	resp := markUnavailableUsage(&ProviderResponse{
		Provider:        req.Provider,
		Error:           reason,
		ExecutedBackend: paneBackendName,
	}, usageSourcePane, usageReasonPane)
	return resp, err
}

// paneProvisioningError identifies failures originating from SplitPane. Callers
// may fall back to subprocess only for this error type.
type paneProvisioningError struct{ cause error }

func (e *paneProvisioningError) Error() string { return e.cause.Error() }
func (e *paneProvisioningError) Unwrap() error { return e.cause }

func newPaneProvisioningError(cause error) error {
	return &paneProvisioningError{cause: cause}
}

func isPaneProvisioningError(err error) bool {
	var target *paneProvisioningError
	return errors.As(err, &target)
}

// subprocessFailed reports whether a subprocess response should be treated as a
// failure (timed out or empty), so we do not pass off an empty fallback as success.
func subprocessFailed(resp *ProviderResponse) bool {
	return resp.TimedOut || resp.EmptyOutput
}
