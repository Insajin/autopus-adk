package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const sessionReadyPollInterval = 200 * time.Millisecond
const promptPollInterval = 200 * time.Millisecond
const promptSubmitDelay = 100 * time.Millisecond

// RunInteractivePaneOrchestra runs interactive CLI orchestration with ReadScreen polling.
// @AX:NOTE [AUTO] interactive orchestration entry point — fan_in=1 (pane_runner.go only); downgraded from ANCHOR
func RunInteractivePaneOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	// R8: plain terminal -> fallback to sentinel mode
	if cfg.Terminal == nil || cfg.Terminal.Name() == "plain" {
		cfg.Interactive = false
		return RunPaneOrchestra(ctx, cfg)
	}

	// Debate strategy with multi-round: delegate to interactive debate loop.
	if cfg.Strategy == StrategyDebate && cfg.DebateRounds >= 2 {
		return runInteractiveDebate(ctx, cfg)
	}

	start := time.Now()
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Hook mode: create session for file-based result collection
	var hookSession *HookSession
	if cfg.HookMode {
		var hsErr error
		hookSession, hsErr = NewHookSession(cfg.SessionID)
		if hsErr != nil {
			// R8: fallback to non-hook mode
			cfg.HookMode = false
		} else {
			defer hookSession.Cleanup()
			_ = os.Setenv("AUTOPUS_SESSION_ID", cfg.SessionID)
		}
	}

	panes, failed, err := splitProviderPanes(timeoutCtx, cfg)
	if err != nil {
		cfg.Interactive = false
		return RunPaneOrchestra(ctx, cfg)
	}
	defer cleanupInteractivePanes(cfg.Terminal, panes)

	if err := startPipeCapture(timeoutCtx, cfg.Terminal, panes); err != nil {
		// Pipe-pane is for idle detection (secondary signal) only.
		// Primary completion uses ReadScreen polling — continue without pipe capture.
		log.Printf("[interactive] startPipeCapture failed: %v -- continuing without idle detection", err)
	}

	launchFailed := launchInteractiveSessions(timeoutCtx, cfg, panes)
	failed = append(failed, launchFailed...)
	readyFailed := waitForSessionReady(timeoutCtx, cfg.Terminal, panes)
	failed = append(failed, readyFailed...)
	promptFailed := sendPrompts(timeoutCtx, cfg, panes)
	failed = append(failed, promptFailed...)

	// REQ-3: configurable initial delay before completion detection (default 20s)
	initialDelay := completionInitialDelay(cfg, 20*time.Second)
	time.Sleep(initialDelay)

	patterns := DefaultCompletionPatterns()
	var responses []ProviderResponse
	if cfg.HookMode && hookSession != nil {
		var hookErr error
		responses, hookErr = WaitAndCollectHookResults(cfg, cfg.SessionID)
		if hookErr != nil {
			responses = waitAndCollectResults(timeoutCtx, cfg, panes, patterns, start, nil, hookSession, 0)
		}
	} else {
		responses = waitAndCollectResults(timeoutCtx, cfg, panes, patterns, start, nil, hookSession, 0)
	}

	// Step 8: Merge by strategy (reuse existing mergeByStrategy)
	total := time.Since(start)
	merged, summary := mergeByStrategy(cfg.Strategy, responses, cfg)
	if merged == "" {
		merged = fmt.Sprintf("[interactive mode] %d providers executed", len(responses))
	}

	return &OrchestraResult{
		Strategy:        cfg.Strategy,
		Responses:       responses,
		Merged:          merged,
		Duration:        total,
		Summary:         summary,
		FailedProviders: failed,
	}, nil
}

// startPipeCapture starts pipe-pane output streaming for each pane.
func startPipeCapture(ctx context.Context, term terminal.Terminal, panes []paneInfo) error {
	for _, pi := range panes {
		if err := term.PipePaneStart(ctx, pi.paneID, pi.outputFile); err != nil {
			return fmt.Errorf("pipe-pane start for %s: %w", pi.provider.Name, err)
		}
	}
	return nil
}

// launchInteractiveSessions launches provider CLIs in each pane using SendLongText (FR-02).
func launchInteractiveSessions(ctx context.Context, cfg OrchestraConfig, panes []paneInfo) []FailedProvider {
	var failed []FailedProvider
	for i, pi := range panes {
		var launchPrompt string
		var promptFile string
		var responseFile string
		if promptDeliveredAtLaunch(pi.provider) {
			launchPrompt, promptFile, responseFile = panePromptText(cfg, pi.provider, 1, cfg.Prompt)
			if promptFile != "" {
				panes[i].promptFiles = append(panes[i].promptFiles, promptFile)
			}
			panes[i].responseFile = responseFile
		}
		cmd := buildInteractiveLaunchCmdWithCWD(pi.provider, launchPrompt, cfg.WorkingDir)
		// FR-02: Use SendLongText for launch command body (handles long args-based prompts)
		if err := cfg.Terminal.SendLongText(ctx, pi.paneID, cmd); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("launch session failed: %v", err),
			})
			panes[i].skipWait = true
			continue
		}
		// Send Enter separately (SendLongText contract: callers send Enter)
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("launch enter failed: %v", err),
			})
			panes[i].skipWait = true
		}
	}
	return failed
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
			Error: fmt.Sprintf("session never became ready after %s (prompt was not sent)", timeout),
		})
	}
	return failed
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
			screen, err := term.ReadScreen(ctx, paneID, terminal.ReadScreenOpts{})
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
			screen, err := term.ReadScreen(ctx, paneID, terminal.ReadScreenOpts{})
			if err != nil {
				continue
			}
			if isSessionReady(screen, patterns) {
				return true
			}
		}
	}
}

// sendPrompts sends the user prompt to each interactive session.
// Sends prompt text first, then a separate Enter to submit (handles paste-mode CLIs).
func sendPrompts(ctx context.Context, cfg OrchestraConfig, panes []paneInfo) []FailedProvider {
	var failed []FailedProvider
	for i, pi := range panes {
		if pi.skipWait {
			continue
		}
		// Skip sendPrompts for providers that received the prompt via CLI args at launch
		if promptDeliveredAtLaunch(pi.provider) {
			continue
		}
		promptText, promptFile, responseFile := panePromptText(cfg, pi.provider, 1, cfg.Prompt)
		if promptFile != "" {
			panes[i].promptFiles = append(panes[i].promptFiles, promptFile)
		}
		panes[i].responseFile = responseFile
		// Send prompt text via SendLongText (uses buffer-based delivery for long prompts)
		if err := cfg.Terminal.SendLongText(ctx, pi.paneID, promptText); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("send prompt failed: %v", err),
			})
			panes[i].skipWait = true
			continue
		}
		// Small delay to let the CLI register the pasted text
		time.Sleep(promptSubmitDelay)
		// Send Enter separately to submit the prompt
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("send enter failed: %v", err),
			})
			panes[i].skipWait = true
		}
	}
	return failed
}

// waitAndCollectResults is in interactive_collect.go.
// waitForCompletion is in interactive_completion.go.
// buildInteractiveLaunchCmd and cleanupInteractivePanes are in interactive_launch.go.
