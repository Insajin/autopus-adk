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
		// Build prompt with topic isolation or context-aware instruction.
		isolation := topicIsolationInstruction
		if cfg.ContextAware {
			isolation = contextAwareInstruction
		}
		var prompt string
		if prevResponses == nil {
			prompt = isolation + cfg.Prompt
		} else {
			var others []ProviderResponse
			for _, r := range prevResponses {
				if r.Provider != pi.provider.Name {
					others = append(others, r)
				}
			}
			prompt = isolation + buildRebuttalPrompt(cfg.Prompt, others, round)
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
			if cfg.ReliabilityStore != nil {
				receipt := promptReceipt(cfg.RunID, pi.provider.Name, "cli_args", prompt, round, "pass", "")
				_ = cfg.ReliabilityStore.recordPrompt(receipt)
			}
			continue
		}

		// File IPC for Round 2+ when hook is available (SPEC-ORCH-017 R4)
		if round > 1 && hookSession != nil && hookSession.HasHook(pi.provider.Name) {
			if tryFileIPC(ctx, hookSession, pi.provider.Name, round, prompt) {
				if cfg.ReliabilityStore != nil {
					receipt := promptReceipt(cfg.RunID, pi.provider.Name, "file_ipc", prompt, round, "pass", "")
					_ = cfg.ReliabilityStore.recordPrompt(receipt)
				}
				continue
			}
			if cfg.ReliabilityStore != nil {
				receipt := promptReceipt(cfg.RunID, pi.provider.Name, "file_ipc", prompt, round, "failed", "file IPC fallback activated before completion wait")
				_ = cfg.ReliabilityStore.recordPrompt(receipt)
			}
		}

		sendPrompt := prompt

		// Sendkeys mode: use SendCommand (cmux send) instead of paste-buffer.
		// Some TUIs (e.g., Codex/ink) display paste-buffer as "[Pasted Content N chars]"
		// instead of processing it as input.
		var newPI paneInfo
		var recreated bool
		var sendErr error
		if pi.provider.InteractiveInput == "sendkeys" {
			// Normalize newlines to spaces for sendkeys (shell line continuation prevention).
			normalized := strings.ReplaceAll(sendPrompt, "\n", " ")
			sendErr = cfg.Terminal.SendCommand(ctx, pi.paneID, normalized)
			newPI = *pi
		} else {
			// R6: On SendLongText failure, attempt pane recreation once, then retry.
			newPI, recreated, sendErr = sendPromptWithRetry(ctx, cfg, *pi, sendPrompt, round, baselines)
		}
		if sendErr != nil {
			log.Printf("[Round %d] %s send failed: %v -- skipping", round, pi.provider.Name, sendErr)
			panes[i].skipWait = true
			if cfg.ReliabilityStore != nil {
				mode := "send_long_text"
				if pi.provider.InteractiveInput == "sendkeys" {
					mode = "sendkeys"
				}
				receipt := promptReceipt(cfg.RunID, pi.provider.Name, mode, sendPrompt, round, "failed", sendErr.Error())
				_ = cfg.ReliabilityStore.recordPrompt(receipt)
			}
			continue
		}
		if cfg.ReliabilityStore != nil {
			mode := "send_long_text"
			if pi.provider.InteractiveInput == "sendkeys" {
				mode = "sendkeys"
			}
			receipt := promptReceipt(cfg.RunID, pi.provider.Name, mode, sendPrompt, round, "pass", "")
			_ = cfg.ReliabilityStore.recordPrompt(receipt)
		}
		if recreated {
			panes[i] = newPI
		}
		submitDelay := 100 * time.Millisecond
		if cfg.HookMode && hookSession != nil {
			submitDelay = 50 * time.Millisecond
		}
		time.Sleep(submitDelay)
		// R8: Retry once on SendCommand (Enter) failure.
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); err != nil {
			log.Printf("[Round %d] %s SendCommand failed: %v — retrying", round, pi.provider.Name, err)
			time.Sleep(1 * time.Second)
			if retryErr := cfg.Terminal.SendCommand(ctx, pi.paneID, "\n"); retryErr != nil {
				log.Printf("[Round %d] %s SendCommand retry failed: %v — skipping", round, pi.provider.Name, retryErr)
				panes[i].skipWait = true
				if cfg.ReliabilityStore != nil {
					receipt := promptReceipt(cfg.RunID, pi.provider.Name, "submit_enter", sendPrompt, round, "failed", retryErr.Error())
					_ = cfg.ReliabilityStore.recordPrompt(receipt)
				}
				continue
			}
		}
	}

	// @AX:NOTE: [AUTO] REQ-3 configurable initial delay — AI processing head start before polling
	debateDelay := completionInitialDelay(cfg, 10*time.Second)
	time.Sleep(debateDelay)

	// Re-capture baselines after the initial delay so poll fallback compares
	// against a fresh snapshot of in-progress output.
	baselines = captureBaselines(ctx, cfg.Terminal, panes)

	// Collect results via hook or screen polling.
	// Use a fresh context — the round context is partially consumed.
	pollTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if pollTimeout <= 0 {
		pollTimeout = 60 * time.Second
	}
	pollCtx, pollCancel := context.WithTimeout(ctx, pollTimeout)
	defer pollCancel()

	var responses []ProviderResponse
	if cfg.HookMode && hookSession != nil {
		responses = collectRoundHookResults(pollCtx, cfg, hookSession, round)
	} else {
		responses = waitAndCollectResults(pollCtx, cfg, panes, patterns, time.Now(), baselines, hookSession, round)
	}
	// R8: Mark providers with empty output for partial merge
	for i := range responses {
		if responses[i].Output == "" && !responses[i].TimedOut {
			responses[i].EmptyOutput = true
		}
	}
	return responses
}
