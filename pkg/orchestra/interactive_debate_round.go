package orchestra

import (
	"context"
	"log"
	"strings"
	"time"
)

// executeRound sends prompts to all panes and collects responses for one round.
func executeRound(ctx context.Context, cfg OrchestraConfig, panes []paneInfo, hookSession *HookSession, round int, prevResponses []ProviderResponse) []ProviderResponse {
	patterns := DefaultCompletionPatterns()

	// R1: Validate surfaces for Round 2+ and recreate stale panes.
	if round > 1 && cfg.SurfaceMgr != nil {
		for i, pi := range panes {
			if pi.skipWait {
				continue
			}
			newPI, recovered, err := cfg.SurfaceMgr.ValidateAndRecover(ctx, cfg, pi, round)
			if err != nil {
				log.Printf("[Round %d] %s recovery failed: %v -- skipping", round, pi.provider.Name, err)
				panes[i].skipWait = true
			} else if recovered {
				panes[i] = newPI
			}
		}
	} else if round > 1 {
		// Fallback: no SurfaceManager -- use direct validation.
		for i, pi := range panes {
			if pi.skipWait {
				continue
			}
			if !validateSurface(ctx, cfg.Terminal, pi.paneID) {
				newPI, err := recreatePane(ctx, cfg, pi, round)
				if err != nil {
					log.Printf("[Round %d] %s surface invalid, recreate failed: %v -- skipping", round, pi.provider.Name, err)
					panes[i].skipWait = true
				} else {
					panes[i] = newPI
				}
			}
		}
	}

	// R2: Capture screen baselines AFTER surface validation/recreation (R7).
	baselines := captureBaselines(ctx, cfg.Terminal, panes)
	for i := range panes {
		pi := &panes[i]
		if pi.skipWait {
			continue
		}
		var prompt string
		if prevResponses == nil {
			prompt = topicIsolationInstruction + cfg.Prompt
		} else {
			var others []ProviderResponse
			for _, r := range prevResponses {
				if r.Provider != pi.provider.Name {
					others = append(others, r)
				}
			}
			prompt = topicIsolationInstruction + buildRebuttalPrompt(cfg.Prompt, others, round)
		}
		if round > 1 {
			// Only send round env to shell-based providers (args mode).
			if pi.provider.InteractiveInput == "args" {
				_ = SendRoundEnvToPane(ctx, cfg.Terminal, pi.paneID, round)
			}
			pollUntilPrompt(ctx, cfg.Terminal, pi.paneID, patterns, round2PollTimeout)
		}

		// Skip sendPrompts for providers that received the prompt via CLI args at launch (round 1 only)
		if pi.provider.InteractiveInput == "args" && round == 1 {
			continue
		}

		// File IPC for Round 2+ when hook is available (SPEC-ORCH-017 R4)
		if round > 1 && hookSession != nil && hookSession.HasHook(pi.provider.Name) {
			if tryFileIPC(ctx, hookSession, pi.provider.Name, round, prompt) {
				continue
			}
		}

		// Normalize newlines for TUI providers (gemini) that treat each line as a separate message.
		sendPrompt := prompt
		if pi.provider.InteractiveInput == "args" {
			sendPrompt = strings.ReplaceAll(prompt, "\n", " ")
		}

		// R6: On SendLongText failure, attempt pane recreation once, then retry.
		newPI, recreated, sendErr := sendPromptWithRetry(ctx, cfg, *pi, sendPrompt, round, baselines)
		if sendErr != nil {
			log.Printf("[Round %d] %s send failed: %v -- skipping", round, pi.provider.Name, sendErr)
			panes[i].skipWait = true
			continue
		}
		if recreated {
			panes[i] = newPI
		}
		time.Sleep(500 * time.Millisecond)
		// R8: Retry once on SendCommand (Enter) failure.
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); err != nil {
			log.Printf("[Round %d] %s SendCommand failed: %v — retrying", round, pi.provider.Name, err)
			time.Sleep(1 * time.Second)
			if retryErr := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); retryErr != nil {
				log.Printf("[Round %d] %s SendCommand retry failed: %v — skipping", round, pi.provider.Name, retryErr)
				panes[i].skipWait = true
				continue
			}
		}
	}

	// Re-capture baselines AFTER prompts are sent to avoid false completion detection.
	baselines = captureBaselines(ctx, cfg.Terminal, panes)

	// @AX:NOTE: [AUTO] REQ-3 configurable initial delay — AI processing head start before polling
	debateDelay := cfg.InitialDelay
	if debateDelay <= 0 {
		debateDelay = 10 * time.Second
	}
	time.Sleep(debateDelay)

	// Re-capture baselines AFTER debateDelay so poll fallback uses a fresh
	// reference that reflects the provider's in-progress output.
	baselines = captureBaselines(ctx, cfg.Terminal, panes)

	// Collect results via hook or screen polling.
	// Use a fresh context — the round context is partially consumed.
	pollTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if pollTimeout <= 0 {
		pollTimeout = 60 * time.Second
	}
	pollCtx, pollCancel := context.WithTimeout(context.Background(), pollTimeout)
	defer pollCancel()

	var responses []ProviderResponse
	if cfg.HookMode && hookSession != nil {
		responses = collectRoundHookResults(pollCtx, cfg, hookSession, round)
	} else {
		responses = waitAndCollectResults(pollCtx, cfg, panes, patterns, time.Now(), baselines, round)
	}
	// R8: Mark providers with empty output for partial merge
	for i := range responses {
		if responses[i].Output == "" && !responses[i].TimedOut {
			responses[i].EmptyOutput = true
		}
	}
	return responses
}
