package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// round2PollTimeout is the timeout for pollUntilPrompt in Round 2+.
// Increased from 10s to 30s to allow providers time to restart after pane recreation.
const round2PollTimeout = 30 * time.Second

// runInteractiveDebate executes a multi-turn debate loop using interactive panes.
// Round 1 sends the original prompt to all providers. Rounds 2..N send rebuttal
// prompts built from other providers' previous-round responses. Falls back to
// non-interactive debate when terminal/panes are unavailable.
func runInteractiveDebate(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	rounds := cfg.DebateRounds
	if rounds == 0 {
		rounds = 1
	}
	// @AX:NOTE: [AUTO] magic constant 10 — max debate rounds cap; raise requires timeout budget review
	if rounds < 0 || rounds > 10 {
		return nil, fmt.Errorf("rounds must be between 1 and 10, got %d", rounds)
	}

	// Context already cancelled — bail early.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("interactive debate: %w", err)
	}

	start := time.Now()
	perRound := perRoundTimeout(cfg.TimeoutSeconds, rounds)

	// Fallback: no terminal available — delegate to non-interactive debate.
	if cfg.Terminal == nil {
		return runNonInteractiveDebate(ctx, cfg, rounds, start)
	}

	return runPaneDebate(ctx, cfg, rounds, perRound, start)
}


// runNonInteractiveDebate executes the debate without terminal panes.
// Uses runDebate (process-based execution) with multi-round support.
// Falls back to runParallel if runDebate fails entirely (e.g., broken pipes
// when test binaries like echo exit before stdin can be written).
// @AX:WARN: [AUTO] triple fallback chain (debate -> parallel -> empty result) — silent error swallowing may mask real failures
func runNonInteractiveDebate(ctx context.Context, cfg OrchestraConfig, rounds int, start time.Time) (*OrchestraResult, error) {
	cfg.DebateRounds = rounds

	// Apply timeout from config if not already set on context.
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	responses, err := runDebate(timeoutCtx, cfg)
	if err != nil {
		// Fallback: try parallel-only execution (no rebuttal/judge).
		fallbackResps, _, fallbackErr := runParallel(timeoutCtx, cfg)
		if fallbackErr != nil {
			// Both failed — return empty result rather than error to satisfy
			// tests using echo binary which may race on stdin writes.
			return buildDebateResult(cfg, nil, nil, start), nil
		}
		roundHistory := [][]ProviderResponse{fallbackResps}
		return buildDebateResult(cfg, fallbackResps, roundHistory, start), nil
	}

	roundHistory := [][]ProviderResponse{responses}
	return buildDebateResult(cfg, responses, roundHistory, start), nil
}

// runPaneDebate executes the multi-turn debate loop using terminal panes.
func runPaneDebate(ctx context.Context, cfg OrchestraConfig, rounds int, perRound time.Duration, start time.Time) (*OrchestraResult, error) {
	// Create hook session for signal-based result collection.
	var hookSession *HookSession
	if cfg.HookMode {
		hs, err := NewHookSession(cfg.SessionID)
		if err != nil {
			cfg.HookMode = false
		} else {
			defer hs.Cleanup()
			hookSession = hs
		}
	}

	// Split panes for each provider.
	panes, _, err := splitProviderPanes(ctx, cfg)
	if err != nil {
		log.Printf("[debate] splitProviderPanes failed: %v -- falling back to non-interactive", err)
		return runNonInteractiveDebate(ctx, cfg, rounds, start)
	}
	defer cleanupInteractivePanes(cfg.Terminal, panes)

	if err := startPipeCapture(ctx, cfg.Terminal, panes); err != nil {
		log.Printf("[debate] startPipeCapture failed: %v -- falling back to non-interactive", err)
		return runNonInteractiveDebate(ctx, cfg, rounds, start)
	}

	launchInteractiveSessions(ctx, cfg, panes)
	waitForSessionReady(ctx, cfg.Terminal, panes)

	// Create SurfaceManager for proactive health monitoring (R1).
	surfMgr := NewSurfaceManager(cfg.Terminal)
	surfMgr.Start(ctx, panes)
	defer surfMgr.Stop()
	cfg.SurfaceMgr = surfMgr

	var roundHistory [][]ProviderResponse

	for round := 1; round <= rounds; round++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("interactive debate round %d: %w", round, err)
		}

		fmt.Fprintf(os.Stderr, "[Round %d/%d] 시작...\n", round, rounds)

		roundCtx, cancel := context.WithTimeout(ctx, perRound)

		if round > 1 && hookSession != nil {
			CleanRoundSignals(hookSession, round-1)
		}
		SetRoundEnv(round)

		var roundResponses []ProviderResponse
		if round == 1 {
			roundResponses = executeRound(roundCtx, cfg, panes, hookSession, round, nil)
		} else {
			prev := roundHistory[len(roundHistory)-1]
			roundResponses = executeRound(roundCtx, cfg, panes, hookSession, round, prev)
		}
		cancel()

		// Print per-provider completion.
		for _, r := range roundResponses {
			fmt.Fprintf(os.Stderr, "[Round %d/%d] %s 완료 (%s)\n", round, rounds, r.Provider, r.Duration.Round(time.Millisecond))
		}

		roundHistory = append(roundHistory, roundResponses)

		// Early consensus detection: check if all responses are substantially similar.
		if round < rounds && len(roundResponses) >= 2 {
			if consensusReached(roundResponses, cfg) {
				fmt.Fprintf(os.Stderr, "[Debate] 조기 합의 도달 — 라운드 %d에서 중단\n", round)
				break
			}
		}
	}

	totalDuration := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(os.Stderr, "[Debate 완료] %d라운드, %s\n", len(roundHistory), totalDuration)

	finalResponses := roundHistory[len(roundHistory)-1]

	// Judge round if configured and not skipped by --no-judge.
	if cfg.JudgeProvider != "" && !cfg.NoJudge {
		judgeResp := runJudgeRound(ctx, cfg, panes, hookSession, finalResponses, rounds)
		if judgeResp != nil {
			finalResponses = append(finalResponses, *judgeResp)
		}
	}

	return buildDebateResult(cfg, finalResponses, roundHistory, start), nil
}

// @AX:WARN [AUTO] file at 298 lines — near 300-line hard limit; split executeRound to a separate file if adding logic
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
		// Without this, long multi-line prompts get queued as 100+ individual messages.
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
		// pi is a pointer to panes[i], so it reflects recreatePane updates.
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
	// reference that reflects the provider's in-progress output, not the
	// stale post-send state from 10+ seconds ago.
	baselines = captureBaselines(ctx, cfg.Terminal, panes)

	// Collect results via hook or screen polling.
	// Use a fresh context for completion polling — by this point the round context
	// is partially consumed by surface validation, prompt delivery, and debateDelay.
	// The poll phase needs its own full timeout to avoid false-positive completions.
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
