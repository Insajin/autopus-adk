package orchestra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

const (
	defaultHookReleaseTimeout = 3 * time.Second
	hookReleasePollInterval   = 20 * time.Millisecond
)

type hookReleaseTarget struct {
	provider  string
	abortName string
	readyName string
}

// releaseYieldHookWaiters returns hook-capable panes to an interactive prompt
// before their ownership is transferred to a durable yield session. All aborts
// are published first, then observed under one shared deadline so a slow hook
// cannot multiply the handoff timeout by the provider count.
func releaseYieldHookWaiters(ctx context.Context, session *HookSession, panes []paneInfo, nextRound int, timeout time.Duration) error {
	if session == nil {
		return nil
	}
	targets, err := yieldHookReleaseTargets(session, panes, nextRound)
	if err != nil {
		return fmt.Errorf("resolve yield hook release targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}
	if timeout <= 0 {
		return fmt.Errorf("yield hook release timeout must be positive")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("release yield hooks: %w", err)
	}
	for _, target := range targets {
		if err := session.WriteAbortSignal(target.provider, nextRound); err != nil {
			return fmt.Errorf("write abort for %s round %d: %w", target.provider, nextRound, err)
		}
	}
	return waitForHookReleaseAcknowledgement(ctx, session, targets, nextRound, timeout)
}

func waitForHookReleaseAcknowledgement(
	ctx context.Context,
	session *HookSession,
	targets []hookReleaseTarget,
	round int,
	timeout time.Duration,
) error {
	if timeout <= 0 {
		return fmt.Errorf("hook release timeout must be positive")
	}
	releaseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := releaseCtx.Err(); err != nil {
		return fmt.Errorf("wait for hook abort consumption in round %d: %w", round, err)
	}

	pending := make(map[string]hookReleaseTarget, len(targets))
	for _, target := range targets {
		pending[target.provider] = target
	}
	ticker := time.NewTicker(hookReleasePollInterval)
	defer ticker.Stop()
	for len(pending) > 0 {
		for provider, target := range pending {
			removed, err := hookReleaseArtifactsRemoved(session, target)
			if err != nil {
				return fmt.Errorf("inspect hook release for %s round %d: %w", provider, round, err)
			}
			if removed {
				delete(pending, provider)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		select {
		case <-releaseCtx.Done():
			providers := make([]string, 0, len(pending))
			for provider := range pending {
				providers = append(providers, provider)
			}
			sort.Strings(providers)
			return fmt.Errorf("wait for hook abort consumption by %v in round %d: %w", providers, round, releaseCtx.Err())
		case <-ticker.C:
		}
	}
	return nil
}

func yieldHookReleaseTargets(session *HookSession, panes []paneInfo, nextRound int) ([]hookReleaseTarget, error) {
	seen := make(map[string]bool, len(panes))
	targets := make([]hookReleaseTarget, 0, len(panes))
	for _, pane := range panes {
		provider := pane.provider.Name
		if seen[provider] || !session.HasHook(provider) {
			continue
		}
		seen[provider] = true
		target := newHookReleaseTarget(provider, nextRound)
		if pane.skipWait {
			info, err := session.statArtifact(target.readyName)
			switch {
			case err == nil && info.Mode().IsRegular():
			case err == nil:
				return nil, fmt.Errorf("inspect skipped hook ready for %s round %d: artifact is not a regular file", provider, nextRound)
			case errors.Is(err, os.ErrNotExist):
				continue
			default:
				return nil, fmt.Errorf("inspect skipped hook ready for %s round %d: %w", provider, nextRound, err)
			}
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func newHookReleaseTarget(provider string, round int) hookReleaseTarget {
	return hookReleaseTarget{
		provider:  provider,
		abortName: RoundSignalName(provider, round, "abort"),
		readyName: RoundSignalName(provider, round, "ready"),
	}
}

func hookReleaseArtifactsRemoved(session *HookSession, target hookReleaseTarget) (bool, error) {
	for _, name := range []string{target.abortName, target.readyName} {
		if _, err := session.statArtifact(name); err == nil {
			return false, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return true, nil
}

func yieldHookReleaseBudget(perRound time.Duration) time.Duration {
	if perRound > 0 && perRound < defaultHookReleaseTimeout {
		return perRound
	}
	return defaultHookReleaseTimeout
}
