package orchestra

import (
	"context"
	"log"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

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
		// Send prompt text via SendLongText (uses buffer-based delivery for long prompts).
		if err := cfg.Terminal.SendLongText(ctx, pi.paneID, promptText); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: "send prompt failed: " + err.Error(),
			})
			panes[i].skipWait = true
			continue
		}
		// Small delay to let the CLI register the pasted text.
		time.Sleep(promptSubmitDelay)
		// Send Enter separately to submit the prompt.
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: "send enter failed: " + err.Error(),
			})
			panes[i].skipWait = true
		}
	}
	return failed
}
