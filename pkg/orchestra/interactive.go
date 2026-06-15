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
			hookSession.ApplyProviderHooks(cfg.Providers)
			defer hookSession.Cleanup()
			// Set on orchestrator process env for subprocesses spawned directly.
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
	failed = append(failed, responseFailuresFromInteractivePanes(panes, responses, timeout, failed)...)

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
		Degraded:        len(failed) > 0,
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
	var wg sync.WaitGroup
	failedCh := make(chan FailedProvider, len(panes)*2)
	for i := range panes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pi := panes[i]
			recordFailure := func(message string) {
				failedCh <- FailedProvider{Name: pi.provider.Name, Error: message}
				panes[i].skipWait = true
			}
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
			cmd, launchFile, err := buildPaneLaunchCommand(cfg.WorkingDir, pi.provider, launchPrompt)
			if err != nil {
				recordFailure(fmt.Sprintf("launch command failed: %v", err))
				return
			}
			if launchFile != "" {
				panes[i].launchFiles = append(panes[i].launchFiles, launchFile)
			}
			// Export AUTOPUS_SESSION_ID to the pane shell BEFORE launching the provider CLI.
			// The orchestrator process env set via os.Setenv is NOT inherited by cmux surfaces
			// (they are independent login shells). SendSessionEnvToPane mirrors the pattern
			// used by SendRoundEnvToPane for AUTOPUS_ROUND. Without this, hook scripts that
			// guard on AUTOPUS_SESSION_ID (e.g., hook-claude-stop.sh:8) exit 0 as a no-op.
			if cfg.HookMode && cfg.SessionID != "" {
				if envErr := SendSessionEnvToPane(ctx, cfg.Terminal, pi.paneID, cfg.SessionID); envErr != nil {
					log.Printf("[interactive] SendSessionEnvToPane for %s failed (non-fatal): %v", pi.provider.Name, envErr)
				} else if enterErr := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); enterErr != nil {
					log.Printf("[interactive] session-env Enter for %s failed (non-fatal): %v", pi.provider.Name, enterErr)
				}
			}

			// FR-02: Use SendLongText for launch command body (handles long args-based prompts)
			if err := cfg.Terminal.SendLongText(ctx, pi.paneID, cmd); err != nil {
				recordFailure(fmt.Sprintf("launch session failed: %v", err))
				return
			}
			// Send Enter separately (SendLongText contract: callers send Enter)
			time.Sleep(promptRegisterDelay)
			if err := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); err != nil {
				recordFailure(fmt.Sprintf("launch enter failed: %v", err))
			}
		}(i)
	}
	wg.Wait()
	close(failedCh)
	failed := make([]FailedProvider, 0)
	for failure := range failedCh {
		failed = append(failed, failure)
	}
	return failed
}

// waitAndCollectResults is in interactive_collect.go.
// waitForCompletion is in interactive_completion.go.
// buildInteractiveLaunchCmd and cleanupInteractivePanes are in interactive_launch.go.
// pollUntilPrompt, pollUntilSessionReady, waitForSessionReady, sendPrompts are in interactive_poll.go.
