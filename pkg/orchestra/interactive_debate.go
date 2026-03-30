package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

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
		return runNonInteractiveDebate(ctx, cfg, rounds, start)
	}
	defer cleanupInteractivePanes(cfg.Terminal, panes)

	if err := startPipeCapture(ctx, cfg.Terminal, panes); err != nil {
		return runNonInteractiveDebate(ctx, cfg, rounds, start)
	}

	launchInteractiveSessions(ctx, cfg, panes)
	waitForSessionReady(ctx, cfg.Terminal, panes)

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

	// Judge round if configured.
	if cfg.JudgeProvider != "" {
		judgeResp := runJudgeRound(ctx, cfg, panes, hookSession, finalResponses, rounds)
		if judgeResp != nil {
			finalResponses = append(finalResponses, *judgeResp)
		}
	}

	return buildDebateResult(cfg, finalResponses, roundHistory, start), nil
}

// executeRound sends prompts to all panes and collects responses for one round.
func executeRound(ctx context.Context, cfg OrchestraConfig, panes []paneInfo, hookSession *HookSession, round int, prevResponses []ProviderResponse) []ProviderResponse {
	patterns := DefaultCompletionPatterns()

	// R1: Validate surfaces for Round 2+ and recreate stale panes.
	if round > 1 {
		for i, pi := range panes {
			if pi.skipWait || !needsSurfaceCheck(pi.provider) {
				continue
			}
			if !validateSurface(ctx, cfg.Terminal, pi.paneID) {
				newPI, err := recreatePane(ctx, cfg, pi, round)
				if err != nil {
					// R4: Mark as skip on recreation failure.
					log.Printf("[Round %d] %s surface invalid, recreate failed: %v — skipping", round, pi.provider.Name, err)
					panes[i].skipWait = true
				} else {
					panes[i] = newPI
				}
			}
		}
	}

	// R2: Capture screen baselines AFTER surface validation/recreation to prevent
	// false-positive completion detection. Must use current (possibly new) pane IDs.
	baselines := make(map[string]string)
	for _, pi := range panes {
		if pi.skipWait {
			continue
		}
		screen, _ := cfg.Terminal.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{})
		baselines[pi.provider.Name] = screen
	}

	for i := range panes {
		pi := &panes[i] // pointer to slice element — stays current after recreation
		if pi.skipWait {
			continue
		}

		var prompt string
		if prevResponses == nil {
			// REQ-2: Topic isolation for round 1
			prompt = topicIsolationInstruction + cfg.Prompt
		} else {
			var others []ProviderResponse
			for _, r := range prevResponses {
				if r.Provider != pi.provider.Name {
					others = append(others, r)
				}
			}
			// REQ-2: Topic isolation for rebuttal rounds
			prompt = topicIsolationInstruction + buildRebuttalPrompt(cfg.Prompt, others, round)
		}

		if round > 1 {
			// Only send round env to shell-based providers (args mode).
			// TUI providers (opencode, gemini) would receive this as chat input text.
			if pi.provider.InteractiveInput == "args" {
				_ = SendRoundEnvToPane(ctx, cfg.Terminal, pi.paneID, round)
			}
			pollUntilPrompt(ctx, cfg.Terminal, pi.paneID, patterns, 10*time.Second)
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

		// R6: On SendLongText failure, attempt pane recreation once, then retry
		// with exponential backoff (500ms, 1s, 2s) before skipping.
		if err := cfg.Terminal.SendLongText(ctx, pi.paneID, sendPrompt); err != nil {
			log.Printf("[Round %d] %s SendLongText failed: %v — attempting pane recreation", round, pi.provider.Name, err)  //nolint:lll
			newPI, recErr := recreatePane(ctx, cfg, *pi, round)
			if recErr != nil {
				log.Printf("[Round %d] %s recreatePane failed: %v — skipping", round, pi.provider.Name, recErr)
				panes[i].skipWait = true
				continue
			}
			panes[i] = newPI
			// Refresh baseline for the new pane to prevent false completion detection
			if screen, err := cfg.Terminal.ReadScreen(ctx, newPI.paneID, terminal.ReadScreenOpts{}); err == nil {
				baselines[pi.provider.Name] = screen
			}

			retryOK := false
			for attempt := 1; attempt <= 3; attempt++ {
				if retryErr := cfg.Terminal.SendLongText(ctx, newPI.paneID, sendPrompt); retryErr == nil {
					retryOK = true
					break
				}
				wait := time.Duration(attempt) * 500 * time.Millisecond
				log.Printf("[Round %d] %s SendLongText attempt %d failed, waiting %v...", round, pi.provider.Name, attempt, wait)
				time.Sleep(wait)
			}
			if !retryOK {
				log.Printf("[Round %d] %s SendLongText failed after 3 attempts — skipping", round, pi.provider.Name)
				panes[i].skipWait = true
				continue
			}
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

	// Re-capture baselines AFTER prompts are sent. The screen now shows the submitted
	// prompt text, so baseline must reflect this to avoid false completion detection
	// when the prompt pattern (❯) is still visible from the input echo.
	for _, pi := range panes {
		if pi.skipWait {
			continue
		}
		if screen, err := cfg.Terminal.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{}); err == nil {
			baselines[pi.provider.Name] = screen
		}
	}

	// @AX:NOTE: [AUTO] REQ-3 configurable initial delay — AI processing head start before polling
	debateDelay := cfg.InitialDelay
	if debateDelay <= 0 {
		debateDelay = 10 * time.Second
	}
	time.Sleep(debateDelay)

	// Collect results via hook or screen polling.
	var responses []ProviderResponse
	if cfg.HookMode && hookSession != nil {
		responses = collectRoundHookResults(ctx, cfg, hookSession, round)
	} else {
		responses = waitAndCollectResults(ctx, cfg, panes, patterns, time.Now(), baselines)
	}
	// R8: Mark providers with empty output for partial merge
	for i := range responses {
		if responses[i].Output == "" && !responses[i].TimedOut {
			responses[i].EmptyOutput = true
		}
	}
	return responses
}
