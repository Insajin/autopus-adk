package experiment

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// StopReason names why a Loop stopped.
type StopReason string

const (
	StopReasonMaxIterations  StopReason = "max-iterations"
	StopReasonCircuitBreaker StopReason = "circuit-breaker"
	StopReasonCancelled      StopReason = "cancelled"
	StopReasonTimeout        StopReason = "timeout"
	StopReasonStepError      StopReason = "step-error"
)

// String returns the stable CLI-safe stop reason.
func (r StopReason) String() string {
	return string(r)
}

// StepResult is the result of one experiment iteration.
type StepResult struct {
	Result   Result
	Improved bool
}

// StepFunc performs one experiment iteration.
type StepFunc func(ctx context.Context, iteration int) (StepResult, error)

// Loop owns experiment iteration and hard-stop policy.
type Loop struct {
	cfg Config
}

// NewLoop creates a loop runner from cfg.
func NewLoop(cfg Config) *Loop {
	return &Loop{cfg: cfg}
}

// Run executes step until a configured hard stop is reached.
func (l *Loop) Run(ctx context.Context, step StepFunc) (ExperimentSummary, StopReason, error) {
	if step == nil {
		return ExperimentSummary{}, StopReasonStepError, fmt.Errorf("experiment step is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx := ctx
	cancel := func() {}
	if l.cfg.ExperimentTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, l.cfg.ExperimentTimeout)
	}
	defer cancel()

	rec := NewRecorder()
	breaker := NewCircuitBreaker(l.cfg.CircuitBreakerN)

	for iteration := 1; ; iteration++ {
		if iteration > l.cfg.MaxIterations {
			return rec.Summary(), StopReasonMaxIterations, nil
		}
		if reason, stopped := stopReasonFromContext(runCtx); stopped {
			return rec.Summary(), reason, nil
		}

		stepResult, err := step(runCtx, iteration)
		if err != nil {
			if reason, stopped := stopReasonFromContext(runCtx); stopped {
				return rec.Summary(), reason, nil
			}
			return rec.Summary(), StopReasonStepError, err
		}

		result := stepResult.Result
		if result.Iteration == 0 {
			result.Iteration = iteration
		}
		if result.Timestamp.IsZero() {
			result.Timestamp = time.Now()
		}
		rec.Record(result)

		breaker.Record(stepResult.Improved)
		if breaker.IsTripped() {
			return rec.Summary(), StopReasonCircuitBreaker, nil
		}
		if reason, stopped := stopReasonFromContext(runCtx); stopped {
			return rec.Summary(), reason, nil
		}
	}
}

func stopReasonFromContext(ctx context.Context) (StopReason, bool) {
	err := ctx.Err()
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return StopReasonTimeout, true
	case errors.Is(err, context.Canceled):
		return StopReasonCancelled, true
	default:
		return "", false
	}
}
