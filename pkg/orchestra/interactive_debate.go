package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
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
	// When yield mode is active, only Round 1 runs — allocate full budget
	// to a single round instead of dividing among all planned rounds.
	effectiveRounds := rounds
	if cfg.YieldRounds && effectiveRounds > 1 {
		effectiveRounds = 1
	}
	perRound := perRoundTimeout(cfg.TimeoutSeconds, effectiveRounds, cfg.NoJudge)

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
// @AX:NOTE: [AUTO] triple fallback chain (debate -> parallel -> empty result) — both errors are logged before returning empty result
func runNonInteractiveDebate(ctx context.Context, cfg OrchestraConfig, rounds int, start time.Time) (*OrchestraResult, error) {
	cfg.RunID = ensureRunID(&cfg)
	cfg.DebateRounds = rounds

	// Apply timeout from config if not already set on context.
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	responses, roundHistory, failed, err := runDebate(timeoutCtx, cfg)
	if err != nil {
		log.Printf("[debate] runDebate failed: %v -- falling back to parallel", err)
		// Fallback: try parallel-only execution (no rebuttal/judge).
		fallbackResps, fallbackFailed, fallbackErr := runParallel(timeoutCtx, cfg)
		if fallbackErr != nil {
			log.Printf("[debate] runParallel also failed: %v -- returning error", fallbackErr)
			return nil, fmt.Errorf("debate failed: %v; fallback failed: %w", err, fallbackErr)
		}
		roundHistory = [][]ProviderResponse{fallbackResps}
		result := buildDebateResult(cfg, fallbackResps, roundHistory, start)
		result.FailedProviders = fallbackFailed
		for i := range result.FailedProviders {
			result.FailedProviders[i].CorrelationRunID = cfg.RunID
		}
		return result, nil
	}

	result := buildDebateResult(cfg, responses, roundHistory, start)
	result.FailedProviders = failed
	for i := range result.FailedProviders {
		result.FailedProviders[i].CorrelationRunID = cfg.RunID
	}
	return result, nil
}

// runPaneDebate executes the multi-turn debate loop using terminal panes.
func runPaneDebate(ctx context.Context, cfg OrchestraConfig, rounds int, perRound time.Duration, start time.Time) (*OrchestraResult, error) {
	cfg.RunID = ensureRunID(&cfg)
	if cfg.ReliabilityStore == nil {
		store, err := newReliabilityStore(cfg.RunID)
		if err == nil {
			cfg.ReliabilityStore = store
		}
	}

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
		if cfg.ReliabilityStore != nil {
			event := ReliabilityEvent{
				SchemaVersion: reliabilitySchemaVersion,
				Timestamp:     time.Now().UTC(),
				Correlation:   roundCorrelation(cfg.RunID, "", 0, 1),
				Kind:          "pane_split_failed",
				Severity:      "error",
				Message:       err.Error(),
				NextStep:      "retry with subprocess fallback",
			}
			_ = cfg.ReliabilityStore.recordEvent(event)
		}
		log.Printf("[debate] splitProviderPanes failed: %v -- falling back to non-interactive", err)
		return runNonInteractiveDebate(ctx, cfg, rounds, start)
	}
	// R5: Skip pane cleanup when yield mode is active — keep panes alive.
	if !cfg.YieldRounds {
		defer cleanupInteractivePanes(cfg.Terminal, panes)
	}

	if err := startPipeCapture(ctx, cfg.Terminal, panes); err != nil {
		// Pipe-pane is for idle detection (secondary signal) only.
		// Primary completion uses ReadScreen polling — continue without pipe capture.
		log.Printf("[debate] startPipeCapture failed: %v -- continuing without idle detection", err)
	}

	launchFailed := launchInteractiveSessions(ctx, cfg, panes)
	if !cfg.HookMode || hookSession == nil {
		waitForSessionReady(ctx, cfg.Terminal, panes)
	}
	if cfg.ReliabilityStore != nil {
		for _, provider := range cfg.Providers {
			_ = cfg.ReliabilityStore.recordPreflight(preflightReceipt(cfg.RunID, cfg, provider))
		}
	}

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

		// R5: Yield mode — output JSON after Round 1 and keep panes alive.
		if cfg.YieldRounds && round == 1 {
			fmt.Fprintf(os.Stderr, "[Debate] yield after round 1/%d\n", rounds)
			sessionID := NewSessionID()
			session := OrchestraSession{
				ID:        sessionID,
				Panes:     make(map[string]string),
				CreatedAt: time.Now(),
			}
			for _, pi := range panes {
				session.Panes[pi.provider.Name] = string(pi.paneID)
			}
			for _, r := range roundResponses {
				session.Rounds = append(session.Rounds, []SessionProviderResponse{{
					Provider: r.Provider, Output: r.Output,
					DurationMs: r.Duration.Milliseconds(), TimedOut: r.TimedOut,
				}})
			}
			_ = SaveSession(session)
			output := BuildYieldOutput(cfg, panes, roundHistory, sessionID)
			_ = WriteYieldOutput(os.Stdout, output)
			// surfMgr.Stop() removed — defer at line 115 handles cleanup.
			// Explicit Stop here caused duplicate WarmPool.Close() calls.
			result := buildDebateResult(cfg, roundResponses, roundHistory, start)
			result.FailedProviders = append(result.FailedProviders, launchFailed...)
			if len(result.FailedProviders) > 0 {
				result.Degraded = true
			}
			for i := range result.FailedProviders {
				result.FailedProviders[i].CorrelationRunID = cfg.RunID
			}
			return result, nil
		}

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

	result := buildDebateResult(cfg, finalResponses, roundHistory, start)
	result.FailedProviders = append(result.FailedProviders, launchFailed...)
	if len(result.FailedProviders) > 0 {
		result.Degraded = true
	}
	for i := range result.FailedProviders {
		result.FailedProviders[i].CorrelationRunID = cfg.RunID
	}
	return result, nil
}

// executeRound is in interactive_debate_round.go.
