package orchestra

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// RunInteractivePaneOrchestra runs orchestration using interactive CLI sessions in terminal panes.
// Each provider's CLI binary is launched as an interactive session, prompts are sent via SendCommand,
// and completion is detected via ReadScreen polling and pipe-pane idle detection.
// Falls back to sentinel-based RunPaneOrchestra if interactive mode fails (R8).
// @AX:NOTE [AUTO] interactive orchestration entry point — fan_in=1 (pane_runner.go only); downgraded from ANCHOR
func RunInteractivePaneOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	// R8: plain terminal -> fallback to sentinel mode
	if cfg.Terminal == nil || cfg.Terminal.Name() == "plain" {
		cfg.Interactive = false
		return RunPaneOrchestra(ctx, cfg)
	}

	start := time.Now()
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Step 1: Split panes (reuse existing splitProviderPanes)
	panes, failed, err := splitProviderPanes(timeoutCtx, cfg)
	if err != nil {
		// R8: fallback on split failure
		cfg.Interactive = false
		return RunPaneOrchestra(ctx, cfg)
	}
	defer cleanupInteractivePanes(cfg.Terminal, panes)

	// Step 2: Start pipe capture for each pane
	if err := startPipeCapture(timeoutCtx, cfg.Terminal, panes); err != nil {
		cfg.Interactive = false
		return RunPaneOrchestra(ctx, cfg)
	}

	// Step 3: Launch interactive sessions (send binary name to each pane)
	launchFailed := launchInteractiveSessions(timeoutCtx, cfg, panes)
	failed = append(failed, launchFailed...)

	// Step 4: Wait for sessions to be ready (prompt visible)
	waitForSessionReady(timeoutCtx, cfg.Terminal, panes)

	// Step 5: Send prompts to each session
	promptFailed := sendPrompts(timeoutCtx, cfg, panes)
	failed = append(failed, promptFailed...)

	// Step 6-7: Wait for completion and collect results
	patterns := DefaultCompletionPatterns()
	responses := waitAndCollectResults(timeoutCtx, cfg, panes, patterns, start)

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

// launchInteractiveSessions sends the provider binary name to each pane to start an interactive session.
// In interactive mode, we launch the bare binary (no -p/-q flags) to get a real CLI session.
func launchInteractiveSessions(ctx context.Context, cfg OrchestraConfig, panes []paneInfo) []FailedProvider {
	var failed []FailedProvider
	for i, pi := range panes {
		// Interactive mode: launch binary alone without print/pipe flags
		// The user prompt will be sent separately via sendPrompts()
		// Append \n to press Enter (cmux send requires explicit \n for Enter)
		cmd := pi.provider.Binary + "\n"
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, cmd); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("launch session failed: %v", err),
			})
			panes[i].skipWait = true
		}
	}
	return failed
}

// waitForSessionReady polls ReadScreen until a prompt is visible or timeout.
func waitForSessionReady(ctx context.Context, term terminal.Terminal, panes []paneInfo) {
	patterns := DefaultCompletionPatterns()
	for _, pi := range panes {
		if pi.skipWait {
			continue
		}
		pollUntilPrompt(ctx, term, pi.paneID, patterns, 30*time.Second)
	}
}

// pollUntilPrompt polls ReadScreen at 500ms intervals until a prompt pattern is detected or timeout.
func pollUntilPrompt(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, patterns []CompletionPattern, timeout time.Duration) bool {
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
			if isPromptVisible(screen, patterns) {
				return true
			}
		}
	}
}

// sendPrompts sends the user prompt to each interactive session.
func sendPrompts(ctx context.Context, cfg OrchestraConfig, panes []paneInfo) []FailedProvider {
	var failed []FailedProvider
	for i, pi := range panes {
		if pi.skipWait {
			continue
		}
		// Append \n to press Enter after prompt (cmux send requires explicit \n)
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, cfg.Prompt+"\n"); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("send prompt failed: %v", err),
			})
			panes[i].skipWait = true
		}
	}
	return failed
}

// waitAndCollectResults waits for each provider to complete and collects cleaned results.
// @AX:WARN [AUTO] concurrent goroutine writes to shared responses slice — guarded by mu sync.Mutex
func waitAndCollectResults(ctx context.Context, cfg OrchestraConfig, panes []paneInfo, patterns []CompletionPattern, start time.Time) []ProviderResponse {
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
			timedOut := !waitForCompletion(ctx, cfg.Terminal, pi, patterns)
			// R9: collect partial results even on timeout
			screen, _ := cfg.Terminal.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{Scrollback: true})
			// R10: clean the output
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

// waitForCompletion polls for completion using both ReadScreen (primary) and idle detection (secondary).
// @AX:NOTE [AUTO] dual detection strategy — prompt pattern (primary) + file idle (secondary, 10s threshold)
func waitForCompletion(ctx context.Context, term terminal.Terminal, pi paneInfo, patterns []CompletionPattern) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false // R9: timeout reached
		case <-ticker.C:
			// Primary: ReadScreen prompt pattern detection (R7)
			screen, err := term.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{})
			if err == nil && isPromptVisible(screen, patterns) {
				return true
			}
			// Secondary: idle detection on pipe-pane output file (R7)
			if isOutputIdle(pi.outputFile, IdleThreshold) {
				return true
			}
		}
	}
}

// cleanupInteractivePanes stops pipe capture and closes panes.
func cleanupInteractivePanes(term terminal.Terminal, panes []paneInfo) {
	ctx := context.Background()
	for _, pi := range panes {
		_ = term.PipePaneStop(ctx, pi.paneID)
		_ = term.Close(ctx, string(pi.paneID))
		_ = os.Remove(pi.outputFile)
	}
}
