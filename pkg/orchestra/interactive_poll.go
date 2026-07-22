package orchestra

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const codexStartupArtifactGrace = time.Second

// waitForPaneReady waits for provider startup evidence and a stable input prompt.
// Hook-ready proves only process startup, so providers with startup hook wiring
// also require two consecutive provider-specific ready frames. Other providers
// use the stable screen gate without waiting for an unavailable artifact.
func waitForPaneReady(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration, hookSession *HookSession, provider string, round int) bool {
	hookRequired := hookSession != nil && hookSession.HasStartupHook(provider)
	hookStarted := !hookRequired
	readyName := ""
	if hookRequired {
		readyName = RoundSignalName(provider, round, "ready")
	}
	const stableReadyFrames = 2
	readyFrames := 0
	var stableSince time.Time
	var artifactMissingSince time.Time
	deadline := time.After(timeout)
	ticker := time.NewTicker(sessionReadyPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline:
			return false
		case <-ticker.C:
			now := time.Now()
			if hookRequired && !hookSession.HasStartupHook(provider) {
				hookRequired = false
				hookStarted = true
			}
			if !hookStarted {
				_, err := hookSession.statArtifact(readyName)
				switch {
				case err == nil:
					hookStarted = true
					artifactMissingSince = time.Time{}
				case errors.Is(err, os.ErrNotExist):
					if artifactMissingSince.IsZero() {
						artifactMissingSince = now
					}
				default:
					artifactMissingSince = time.Time{}
				}
			}
			screen, err := readScreenBounded(ctx, term, paneID, terminal.ReadScreenOpts{})
			if err != nil || !isProviderSessionReady(screen, patterns, provider) {
				readyFrames = 0
				stableSince = time.Time{}
				continue
			}
			if readyFrames == 0 {
				stableSince = now
			}
			readyFrames++
			if hookStarted && readyFrames >= stableReadyFrames {
				return true
			}
			if hookRequired && readyFrames >= stableReadyFrames &&
				providerArtifactIdentity(provider) == "codex" &&
				!artifactMissingSince.IsZero() {
				graceStart := stableSince
				if artifactMissingSince.After(graceStart) {
					graceStart = artifactMissingSince
				}
				if now.Sub(graceStart) >= codexStartupArtifactGrace &&
					hookSession.deactivateCodexStartupHook(provider, round) {
					log.Printf("[startup] %s ready artifact inactive after stable prompt grace; using direct readiness for this run", provider)
					return true
				}
			}
		}
	}
}

// pollUntilPrompt polls ReadScreen at short intervals until a prompt pattern is detected or timeout.
func pollUntilPrompt(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration) bool {
	startTime := time.Now()
	deadline := time.After(timeout)
	ticker := time.NewTicker(promptPollInterval)
	defer ticker.Stop()

	warned := false
	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline:
			return false
		case <-ticker.C:
			if !warned && time.Since(startTime) > 20*time.Second {
				log.Printf("[pollUntilPrompt] %s exceeding 20s threshold, still waiting...", paneID)
				warned = true
			}
			screen, err := readScreenBounded(ctx, term, paneID, terminal.ReadScreenOpts{})
			if err != nil {
				continue
			}
			if isPromptVisible(screen, patterns) {
				return true
			}
		}
	}
}

// pollUntilSessionReady polls ReadScreen at short intervals until a session-ready
// pattern is detected or timeout. Unlike pollUntilPrompt, this uses isSessionReady
// which excludes shell prompts to prevent false session-ready detection.
func pollUntilSessionReady(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(sessionReadyPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline:
			return false
		case <-ticker.C:
			screen, err := readScreenBounded(ctx, term, paneID, terminal.ReadScreenOpts{})
			if err != nil {
				continue
			}
			if isSessionReady(screen, patterns) {
				return true
			}
		}
	}
}

// waitForSessionReady preserves the hookless startup-gate helper contract.
func waitForSessionReady(ctx context.Context, term terminal.Terminal, panes []paneInfo) []FailedProvider {
	return waitForSessionReadyWithHook(ctx, term, panes, nil, 0)
}

// waitForSessionReadyWithHook requires provider-specific, stable screen readiness.
// Providers with startup hook wiring must also emit their ready artifact. Those
// that never become ready are marked skipWait so their prompt is never sent.
func waitForSessionReadyWithHook(ctx context.Context, term terminal.Terminal, panes []paneInfo, hookSession *HookSession, round int) []FailedProvider {
	patterns := SessionReadyPatterns()
	var failed []FailedProvider
	for i, pi := range panes {
		if pi.skipWait {
			continue
		}
		if promptDeliveredAtLaunch(pi.provider) {
			continue
		}
		timeout := startupTimeoutFor(pi.provider)
		if waitForPaneReady(ctx, term, pi.paneID, patterns, timeout, hookSession, pi.provider.Name, round) {
			continue
		}
		panes[i].skipWait = true
		failed = append(failed, FailedProvider{
			Name:  pi.provider.Name,
			Error: "session never became ready after " + timeout.String() + " (prompt was not sent)",
		})
	}
	return failed
}

// sendPrompts sends the user prompt to each interactive session.
// Sends prompt text first, then a separate Enter to submit (handles paste-mode CLIs).
func sendPrompts(ctx context.Context, cfg OrchestraConfig, panes []paneInfo) []FailedProvider {
	var failed []FailedProvider
	for i, pi := range panes {
		if pi.skipWait {
			continue
		}
		// Skip sendPrompts for providers that received the prompt via CLI args at launch.
		if promptDeliveredAtLaunch(pi.provider) {
			continue
		}
		promptText, promptFile, responseFile := panePromptText(cfg, pi.provider, 1, cfg.Prompt)
		if promptFile != "" {
			panes[i].promptFiles = append(panes[i].promptFiles, promptFile)
		}
		panes[i].responseFile = responseFile
		// Commit prompt text and Enter as one cmux input transaction. Separate
		// provider surfaces otherwise share cmux's asynchronous input queue.
		sendErr, enterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, pi.paneID, promptSubmitDelay, func() error {
			return cfg.Terminal.SendLongText(ctx, pi.paneID, promptText)
		})
		if sendErr != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: "send prompt failed: " + sendErr.Error(),
			})
			panes[i].skipWait = true
			continue
		}
		if enterErr != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: "send enter failed: " + enterErr.Error(),
			})
			panes[i].skipWait = true
		}
	}
	return failed
}
