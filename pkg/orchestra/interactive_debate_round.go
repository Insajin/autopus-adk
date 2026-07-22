package orchestra

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"time"
)

// executeRound sends prompts to all panes and collects responses for one round.
func executeRound(ctx context.Context, cfg OrchestraConfig, panes []paneInfo, hookSession *HookSession, round int, prevResponses []ProviderResponse) []ProviderResponse {
	patterns := DefaultCompletionPatterns()
	var roundPromptFiles []string
	var roundResponseFiles []string
	for _, pi := range panes {
		if pi.responseFile != "" {
			roundResponseFiles = append(roundResponseFiles, pi.responseFile)
		}
	}
	defer func() {
		cleanupPromptFiles(roundPromptFiles)
		cleanupPromptFiles(roundResponseFiles)
	}()

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
				roundErr, roundEnterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, pi.paneID, 0, func() error {
					return SendRoundEnvToPane(ctx, cfg.Terminal, pi.paneID, round)
				})
				if roundErr != nil || roundEnterErr != nil {
					log.Printf("[Round %d] %s SendRoundEnvToPane failed: send=%v enter=%v", round, pi.provider.Name, roundErr, roundEnterErr)
				}
			}
			if !pollUntilPrompt(ctx, cfg.Terminal, pi.paneID, patterns, round2PollTimeout) {
				log.Printf("[Round %d] %s prompt not ready within timeout -- skipping", round, pi.provider.Name)
				panes[i].skipWait = true
				if cfg.ReliabilityStore != nil {
					receipt := promptReceipt(cfg.RunID, pi.provider.Name, "prompt_ready", prompt, round, "failed", "prompt not ready before round submission")
					_ = cfg.ReliabilityStore.recordPrompt(receipt)
				}
				continue
			}
		}

		// Skip direct prompt send for providers that received the prompt at launch (round 1 only).
		if promptDeliveredAtLaunch(pi.provider) && round == 1 {
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

		sendPrompt, promptFile, responseFile := panePromptText(cfg, pi.provider, round, prompt)
		if promptFile != "" {
			roundPromptFiles = append(roundPromptFiles, promptFile)
		}
		if responseFile != "" {
			roundResponseFiles = append(roundResponseFiles, responseFile)
		}

		// Sendkeys mode: use SendCommand (cmux send) instead of paste-buffer.
		// Some TUIs (e.g., Codex/ink) display paste-buffer as "[Pasted Content N chars]"
		// instead of processing it as input.
		var newPI paneInfo
		var recreated bool
		var sendErr error
		submitDelay := panePromptSubmitDelay(pi.provider)
		if cfg.HookMode && hookSession != nil && !isCodexInteractiveProvider(pi.provider) {
			submitDelay = 50 * time.Millisecond
		}
		if shouldUseSendkeysPromptInput(pi.provider, promptFile != "") {
			// Normalize newlines to spaces for sendkeys (shell line continuation prevention).
			normalized := strings.ReplaceAll(sendPrompt, "\n", " ")
			var enterErr error
			sendErr, enterErr = sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, pi.paneID, submitDelay, func() error {
				return cfg.Terminal.SendCommand(ctx, pi.paneID, normalized)
			}, time.Second)
			if enterErr != nil {
				sendErr = &paneSubmitEnterError{err: enterErr}
			}
			newPI = *pi
		} else {
			// R6: retry failed prompt commits before recreating the pane. Each
			// successful paste includes its Enter inside the same cmux transaction.
			newPI, recreated, sendErr = sendPromptWithRetry(ctx, cfg, *pi, sendPrompt, round, baselines, submitDelay)
		}
		if promptFile != "" {
			if recreated {
				newPI.promptFiles = append(newPI.promptFiles, promptFile)
			} else {
				pi.promptFiles = append(pi.promptFiles, promptFile)
				panes[i].promptFiles = pi.promptFiles
			}
		}
		if responseFile != "" {
			if recreated {
				newPI.responseFile = responseFile
			} else {
				pi.responseFile = responseFile
				panes[i].responseFile = responseFile
			}
		}
		if sendErr != nil {
			if recreated {
				panes[i] = newPI
			}
			log.Printf("[Round %d] %s send failed: %v -- skipping", round, pi.provider.Name, sendErr)
			panes[i].skipWait = true
			if cfg.ReliabilityStore != nil {
				mode := "send_long_text"
				var enterFailure *paneSubmitEnterError
				if errors.As(sendErr, &enterFailure) {
					mode = "submit_enter"
				}
				if shouldUseSendkeysPromptInput(pi.provider, promptFile != "") {
					if mode != "submit_enter" {
						mode = "sendkeys"
					}
				}
				receipt := promptReceipt(cfg.RunID, pi.provider.Name, mode, sendPrompt, round, "failed", sendErr.Error())
				_ = cfg.ReliabilityStore.recordPrompt(receipt)
			}
			continue
		}
		if cfg.ReliabilityStore != nil {
			mode := "send_long_text"
			if shouldUseSendkeysPromptInput(pi.provider, promptFile != "") {
				mode = "sendkeys"
			}
			receipt := promptReceipt(cfg.RunID, pi.provider.Name, mode, sendPrompt, round, "pass", "")
			_ = cfg.ReliabilityStore.recordPrompt(receipt)
		}
		if recreated {
			panes[i] = newPI
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
		var pollPanes []paneInfo
		for _, pi := range panes {
			if pi.skipWait || hookSession.HasHook(pi.provider.Name) {
				continue
			}
			pollPanes = append(pollPanes, pi)
		}
		if len(pollPanes) > 0 {
			responses = append(responses, waitAndCollectResults(pollCtx, cfg, pollPanes, patterns, time.Now(), baselines, nil, round)...)
		}
	} else {
		responses = waitAndCollectResults(pollCtx, cfg, panes, patterns, time.Now(), baselines, hookSession, round)
	}
	// R8: Mark providers with empty output for partial merge
	for i := range responses {
		if responses[i].Output == "" && !responses[i].TimedOut {
			responses[i].EmptyOutput = true
		}
	}
	return orderDebateResponses(responses, cfg.Providers)
}

func orderDebateResponses(responses []ProviderResponse, providers []ProviderConfig) []ProviderResponse {
	providerOrder := make(map[string]int, len(providers))
	for i, provider := range providers {
		if _, exists := providerOrder[provider.Name]; !exists {
			providerOrder[provider.Name] = i
		}
	}
	sort.SliceStable(responses, func(i, j int) bool {
		left, leftKnown := providerOrder[responses[i].Provider]
		right, rightKnown := providerOrder[responses[j].Provider]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown {
			return left < right
		}
		return responses[i].Provider < responses[j].Provider
	})
	return responses
}
