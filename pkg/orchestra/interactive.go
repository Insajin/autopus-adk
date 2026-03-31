package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

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
		cfg.Interactive = false
		return RunPaneOrchestra(ctx, cfg)
	}

	launchFailed := launchInteractiveSessions(timeoutCtx, cfg, panes)
	failed = append(failed, launchFailed...)
	waitForSessionReady(timeoutCtx, cfg.Terminal, panes)
	promptFailed := sendPrompts(timeoutCtx, cfg, panes)
	failed = append(failed, promptFailed...)

	// REQ-3: configurable initial delay before completion detection (default 20s)
	initialDelay := cfg.InitialDelay
	if initialDelay <= 0 {
		initialDelay = 20 * time.Second
	}
	time.Sleep(initialDelay)

	patterns := DefaultCompletionPatterns()
	var responses []ProviderResponse
	if cfg.HookMode && hookSession != nil {
		var hookErr error
		responses, hookErr = WaitAndCollectHookResults(cfg, cfg.SessionID)
		if hookErr != nil {
			responses = waitAndCollectResults(timeoutCtx, cfg, panes, patterns, start, nil, 0)
		}
	} else {
		responses = waitAndCollectResults(timeoutCtx, cfg, panes, patterns, start, nil, 0)
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
		if pi.provider.InteractiveInput == "args" {
			launchPrompt = cfg.Prompt
		}
		cmd := buildInteractiveLaunchCmd(pi.provider, launchPrompt)
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
func waitForSessionReady(ctx context.Context, term terminal.Terminal, panes []paneInfo) {
	patterns := SessionReadyPatterns()
	for _, pi := range panes {
		if pi.skipWait {
			continue
		}
		timeout := startupTimeoutFor(pi.provider)
		pollUntilSessionReady(ctx, term, pi.paneID, patterns, timeout)
	}
}

// pollUntilPrompt polls ReadScreen at 3s intervals until a prompt pattern is detected or timeout.
// R2: 3s polling interval (was 500ms), warns if exceeding 20s threshold.
func pollUntilPrompt(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration) bool {
	startTime := time.Now()
	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second) // R2: 3s polling interval
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

// pollUntilSessionReady polls ReadScreen at 500ms intervals until a session-ready
// pattern is detected or timeout. Unlike pollUntilPrompt, this uses isSessionReady
// which excludes shell prompts to prevent false session-ready detection.
func pollUntilSessionReady(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
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
		if pi.provider.InteractiveInput == "args" {
			continue
		}
		// Send prompt text via SendLongText (uses buffer-based delivery for long prompts)
		if err := cfg.Terminal.SendLongText(ctx, pi.paneID, cfg.Prompt); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("send prompt failed: %v", err),
			})
			panes[i].skipWait = true
			continue
		}
		// Small delay to let the CLI register the pasted text
		time.Sleep(500 * time.Millisecond)
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

// waitAndCollectResults waits for completion and collects cleaned results.
// Round is forwarded to waitForCompletion; pass 0 for non-debate strategies.
// @AX:WARN [AUTO] concurrent goroutine writes to shared responses slice — guarded by mu sync.Mutex
func waitAndCollectResults(ctx context.Context, cfg OrchestraConfig, panes []paneInfo, patterns []CompletionPattern, start time.Time, baselines map[string]string, round int) []ProviderResponse {
	var (
		responses []ProviderResponse
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	for _, pi := range panes {
		if pi.skipWait {
			responses = append(responses, ProviderResponse{
				Provider: pi.provider.Name,
				Duration: time.Since(start),
				TimedOut: true,
			})
			continue
		}
		wg.Add(1)
		go func(pi paneInfo) {
			defer wg.Done()
			var baseline string
			if baselines != nil {
				baseline = baselines[pi.provider.Name]
			}
			timedOut := !waitForCompletion(ctx, cfg.Terminal, pi, patterns, baseline, round)
			// Fresh context for final read — original ctx may be cancelled after timeout.
			readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer readCancel()
			screen, _ := cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{
				Scrollback:      true,
				ScrollbackLines: scrollbackDepth(cfg.ScrollbackLines),
			})
			output := cleanScreenOutput(screen)

			mu.Lock()
			defer mu.Unlock()
			responses = append(responses, ProviderResponse{
				Provider: pi.provider.Name,
				Output:   output,
				Duration: time.Since(start),
				TimedOut: timedOut,
			})
		}(pi)
	}
	wg.Wait()
	return responses
}

// waitForCompletion is in interactive_completion.go.
// buildInteractiveLaunchCmd and cleanupInteractivePanes are in interactive_launch.go.
