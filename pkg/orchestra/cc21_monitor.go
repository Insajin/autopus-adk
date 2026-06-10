package orchestra

import (
	"context"
	"log"
	"time"
)

const monitorInitialDelay = 500 * time.Millisecond

type resolvedCompletionDetector struct {
	detector    CompletionDetector
	eventDriven bool
}

func resolveCompletionDetector(cfg OrchestraConfig, hookSession *HookSession) resolvedCompletionDetector {
	if cfg.CompletionDetector != nil {
		_, isPoll := cfg.CompletionDetector.(*ScreenPollDetector)
		return resolvedCompletionDetector{
			detector:    cfg.CompletionDetector,
			eventDriven: !isPoll,
		}
	}
	// SPEC-ORCH-022: when a hook session is active the done-file IPC detector is the
	// authoritative completion contract and the completion floor. Select it first and
	// as a full-budget wait (eventDriven=false): it must be neither gated on the CC21
	// monitor feature flag nor capped by the short monitor pattern timeout with a
	// screen-poll fallback. The monitor-gated path returned the instant the response
	// rendered on screen, letting the deferred session-dir cleanup (pane_backend.go's
	// `defer hookSession.Cleanup()`) race the provider's Stop hook — the done file was
	// written into an already-removed directory and never collected (the 0/N timeout →
	// screen-scrape fallback this SPEC closes). A full-ctx wait keeps Execute blocked
	// until the done file appears, so the session directory stays alive for the hook.
	if cfg.HookMode && hookSession != nil {
		return resolvedCompletionDetector{
			detector:    &FileIPCDetector{session: hookSession},
			eventDriven: false,
		}
	}
	if cfg.MonitorEnabled {
		if detector, ok := monitorCompletionDetector(cfg, hookSession); ok {
			return resolvedCompletionDetector{
				detector:    detector,
				eventDriven: true,
			}
		}
	}
	return resolvedCompletionDetector{
		detector:    &ScreenPollDetector{term: cfg.Terminal},
		eventDriven: false,
	}
}

func monitorCompletionDetector(cfg OrchestraConfig, hookSession *HookSession) (CompletionDetector, bool) {
	if cfg.Terminal == nil {
		return nil, false
	}
	if detector := NewCompletionDetectorWithConfig(cfg.Terminal, cfg.HookMode, hookSession); detector != nil {
		if _, isPoll := detector.(*ScreenPollDetector); !isPoll {
			return detector, true
		}
	}
	return nil, false
}

func completionInitialDelay(cfg OrchestraConfig, fallback time.Duration) time.Duration {
	if cfg.InitialDelay > 0 {
		return cfg.InitialDelay
	}
	if cfg.MonitorEnabled {
		return monitorInitialDelay
	}
	return fallback
}

func monitorWaitContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func waitForCompletion(ctx context.Context, cfg OrchestraConfig, pi paneInfo, patterns []CompletionPattern, baseline string, hookSession *HookSession, round int) bool {
	resolved := resolveCompletionDetector(cfg, hookSession)
	if !resolved.eventDriven {
		completed, _ := resolved.detector.WaitForCompletion(ctx, pi, patterns, baseline, round)
		return completed
	}

	monitorCtx, cancel := monitorWaitContext(ctx, cfg.MonitorTimeout)
	defer cancel()

	completed, _ := resolved.detector.WaitForCompletion(monitorCtx, pi, patterns, baseline, round)
	if completed || ctx.Err() != nil {
		return completed
	}

	log.Printf("[completion] monitor timeout for %s after %s -- falling back to polling", pi.provider.Name, cfg.MonitorTimeout)
	fallback := &ScreenPollDetector{term: cfg.Terminal}
	completed, _ = fallback.WaitForCompletion(ctx, pi, patterns, baseline, round)
	return completed
}
