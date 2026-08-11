package cli

import (
	"context"
	"errors"
	"sort"
)

// Settlement is separated from dispatch because it has to hold under
// conditions the happy path never sees: a cancelled context, a worker that may
// still be alive, and a Close racing an in-flight attempt. Every exit from the
// backend lands in this file, and every dispatch it started leaves through it
// exactly once.

// Close settles every dispatch this backend started and never settled. It is
// idempotent and keeps going after an individual settlement failure.
func (backend *pipelineOrcaBackend) Close() error {
	backend.mu.Lock()
	if backend.closed {
		backend.mu.Unlock()
		return nil
	}
	backend.closed = true
	pending := make([]string, 0, len(backend.pending))
	for dispatchID := range backend.pending {
		pending = append(pending, dispatchID)
	}
	backend.pending = map[string]struct{}{}
	backend.mu.Unlock()
	sort.Strings(pending)
	settleCtx, cancel := pipelineOrcaCleanupContext(context.Background())
	defer cancel()
	var err error
	for _, dispatchID := range pending {
		err = errors.Join(err, backend.fenceLiveDispatch(settleCtx, dispatchID))
	}
	return err
}

// settleDispatch closes one dispatch exactly once. A worker that reported
// worker_done is released; anything else is fenced, because a timed-out or
// cancelled worker may still be live and release would falsely claim its
// process was stopped.
func (backend *pipelineOrcaBackend) settleDispatch(
	ctx context.Context,
	dispatchID string,
	settled bool,
) error {
	if !backend.claimDispatch(dispatchID) {
		return nil
	}
	settleCtx, cancel := pipelineOrcaCleanupContext(ctx)
	defer cancel()
	if settled {
		_, err := backend.config.Client.Release(settleCtx, dispatchID)
		return err
	}
	return backend.fenceLiveDispatch(settleCtx, dispatchID)
}

// fenceLiveDispatch settles a dispatch whose worker cannot be claimed to have
// stopped. Stopping comes first because abandon fences the dispatch but leaves
// the agent running in this worktree, where it keeps editing files and
// spending tokens. Abandon is still the floor: a stop that fails must not
// leave the dispatch unsettled.
func (backend *pipelineOrcaBackend) fenceLiveDispatch(ctx context.Context, dispatchID string) error {
	if _, err := backend.config.Client.Stop(ctx, dispatchID); err == nil {
		return nil
	}
	_, err := backend.config.Client.Abandon(ctx, dispatchID)
	return err
}

func (backend *pipelineOrcaBackend) trackDispatch(dispatchID string) {
	if dispatchID == "" {
		return
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.pending[dispatchID] = struct{}{}
}

// claimDispatch takes ownership of settling one dispatch. It succeeds for
// exactly one caller, so a Close racing an in-flight attempt cannot settle the
// same dispatch twice.
func (backend *pipelineOrcaBackend) claimDispatch(dispatchID string) bool {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if _, tracked := backend.pending[dispatchID]; !tracked {
		return false
	}
	delete(backend.pending, dispatchID)
	return true
}

// pipelineOrcaCleanupContext detaches cleanup from the caller's cancellation.
// A cancelled run is exactly when settlement matters most, so inheriting that
// cancellation would abort the calls that release the worker.
func pipelineOrcaCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), pipelineOrcaSettleTimeout)
}
