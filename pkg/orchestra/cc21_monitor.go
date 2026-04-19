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
