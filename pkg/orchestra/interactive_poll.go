package orchestra

import (
	"context"
	"log"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// waitForPaneReady waits for provider startup evidence and a stable input prompt.
// Hook-ready proves only process startup, so hook-capable providers also require
// two consecutive provider-specific ready frames. Hookless providers use the
// same stable screen gate without waiting for an unavailable hook artifact.
func waitForPaneReady(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration, hookSession *HookSession, provider string, round int) bool {
	hookRequired := hookSession != nil && hookSession.HasHook(provider)
	hookStarted := !hookRequired
	readyName := ""
	if hookRequired {
		readyName = RoundSignalName(provider, round, "ready")
	}
	const stableReadyFrames = 2
	readyFrames := 0
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
			if !hookStarted {
				if _, err := hookSession.statArtifact(readyName); err == nil {
					hookStarted = true
				}
			}
			screen, err := readScreenBounded(ctx, term, paneID, terminal.ReadScreenOpts{})
			if err != nil || !isProviderSessionReady(screen, patterns, provider) {
				readyFrames = 0
				continue
			}
			readyFrames++
			if hookStarted && readyFrames >= stableReadyFrames {
				return true
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

// waitForSessionReady polls ReadScreen until a CLI-specific prompt is visible or timeout.
// Uses SessionReadyPatterns (no shell $ / # patterns) to avoid false positives.
// Providers that never become ready are marked skipWait so prompts are not sent
// into a shell or half-launched TUI.
func waitForSessionReady(ctx context.Context, term terminal.Terminal, panes []paneInfo) []FailedProvider {
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
		if pollUntilSessionReady(ctx, term, pi.paneID, patterns, timeout) {
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
