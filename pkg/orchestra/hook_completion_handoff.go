package orchestra

import (
	"context"
	"fmt"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const (
	hookStableIdleFrames = 2
)

func needsHookCompletionHandoff(cfg OrchestraConfig, round int) bool {
	if cfg.YieldRounds {
		return true
	}
	rounds := cfg.DebateRounds
	if rounds <= 0 {
		rounds = 1
	}
	return round < rounds
}

// resolveHookCompletionHandoff classifies Codex's grace-based response-only
// completion before another round or yield can interact with its Stop hook.
// Only two stable provider-specific idle frames may deactivate the hook.
// Response-file availability is not turn completion, so working and blocker
// frames remain under observation until next-ready, stable idle, or the
// provider deadline. I/O and timeout outcomes remain active by default.
func resolveHookCompletionHandoff(
	ctx context.Context,
	cfg OrchestraConfig,
	session *HookSession,
	provider ProviderConfig,
	pi paneInfo,
	round int,
) (hookCompletionProvenance, error) {
	if !needsHookCompletionHandoff(cfg, round) || !isCodexInteractiveProvider(provider) {
		return hookCompletionUnknown, nil
	}
	if provenance, err := session.completionArtifactProvenance(provider, round); err != nil ||
		handoffArtifactResolved(provenance) {
		return provenance, err
	}
	if cfg.Terminal == nil {
		return hookCompletionUnknown, fmt.Errorf("completion handoff terminal is unavailable")
	}

	handoffCtx := ctx
	cancel := func() {}
	if _, bounded := ctx.Deadline(); !bounded {
		handoffCtx, cancel = context.WithTimeout(ctx, fileIPCReadyTimeout)
	}
	defer cancel()
	ticker := time.NewTicker(sessionReadyPollInterval)
	defer ticker.Stop()
	stableFrames := 0
	patterns := SessionReadyPatterns()
	for {
		select {
		case <-handoffCtx.Done():
			provenance, err := session.completionArtifactProvenance(provider, round)
			if err != nil || handoffArtifactResolved(provenance) {
				return provenance, err
			}
			return hookCompletionUnknown, fmt.Errorf("completion handoff unresolved: %w", handoffCtx.Err())
		case <-ticker.C:
			provenance, err := session.completionArtifactProvenance(provider, round)
			if err != nil || handoffArtifactResolved(provenance) {
				return provenance, err
			}
			screen, err := readScreenBounded(handoffCtx, cfg.Terminal, pi.paneID, terminal.ReadScreenOpts{})
			if err != nil {
				provenance, artifactErr := session.completionArtifactProvenance(provider, round)
				if artifactErr == nil && handoffArtifactResolved(provenance) {
					return provenance, nil
				}
				if artifactErr != nil {
					return hookCompletionUnknown, fmt.Errorf(
						"read completion handoff screen: %v; recheck completion artifact: %w",
						err, artifactErr,
					)
				}
				return hookCompletionUnknown, fmt.Errorf("read completion handoff screen: %w", err)
			}
			if isSessionReadyBlocked(screen, provider.Name) {
				stableFrames = 0
				continue
			}
			if isProviderWorking(screen) || isProviderStillWorking(screen, provider.WorkingPatterns) {
				stableFrames = 0
				continue
			}
			if !isProviderSessionReady(screen, patterns, provider.Name) {
				stableFrames = 0
				continue
			}
			stableFrames++
			if stableFrames >= hookStableIdleFrames {
				return session.deactivateContinuationHook(provider, round)
			}
		}
	}
}

func handoffArtifactResolved(provenance hookCompletionProvenance) bool {
	return provenance == hookCompletionNextRoundReady
}
