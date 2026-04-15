package orchestra

import (
	"context"
	"fmt"
	"os"
	"time"
)

// collectRoundHookResults collects hook-based results for a specific round.
// @AX:NOTE: [AUTO] magic constant 60s default timeout — per-provider wait; overridden by cfg.TimeoutSeconds
func collectRoundHookResults(ctx context.Context, cfg OrchestraConfig, session *HookSession, round int) []ProviderResponse {
	timeout := 60 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	var responses []ProviderResponse
	for _, p := range cfg.Providers {
		// Respect context cancellation between provider iterations.
		if ctx.Err() != nil {
			break
		}
		start := time.Now()
		err := session.WaitForDoneRoundCtx(ctx, timeout, p.Name, round)
		if err != nil {
			responses = append(responses, ProviderResponse{
				Provider: p.Name,
				Duration: time.Since(start),
				TimedOut: true,
			})
			continue
		}
		result, readErr := session.ReadResultRound(p.Name, round)
		output := ""
		if readErr == nil && result != nil {
			output = result.Output
		}
		responses = append(responses, ProviderResponse{
			Provider: p.Name,
			Output:   output,
			Duration: time.Since(start),
		})
	}
	return responses
}

// runJudgeRound executes the judge verdict after all debate rounds.
// Always runs judge as a non-interactive subprocess for reliable completion detection.
// Uses a fresh context with 120s timeout since the parent context may be near expiry
// after debate rounds consumed most of the allotted time.
// R1: cmd.Run() return (process exit event) is the primary completion signal;
// the context timeout is a safety net only — judge completion is event-based, not poll-based.
func runJudgeRound(ctx context.Context, cfg OrchestraConfig, _ []paneInfo, _ *HookSession, responses []ProviderResponse, _ int) *ProviderResponse {
	judgment := buildJudgmentPrompt(cfg.Prompt, responses)
	judgeCfg := findOrBuildJudgeConfig(cfg)

	// Bound the judge with both the parent context and a local timeout.
	judgeTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if judgeTimeout < 60*time.Second {
		judgeTimeout = 60 * time.Second
	}
	judgeCtx, cancel := context.WithTimeout(ctx, judgeTimeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "[Judge] subprocess 실행 중 (provider: %s, timeout: %s)...\n", cfg.JudgeProvider, judgeTimeout)
	resp, err := runProvider(judgeCtx, judgeCfg, judgment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Judge] 프로세스 실행 실패: %v\n", err)
		return nil
	}
	if resp == nil || resp.TimedOut {
		fmt.Fprintf(os.Stderr, "[Judge] timeout 또는 취소로 판정 생략\n")
		return nil
	}
	if resp.EmptyOutput {
		fmt.Fprintf(os.Stderr, "[Judge] 빈 출력으로 판정 생략\n")
		return nil
	}
	fmt.Fprintf(os.Stderr, "[Judge] 판정 완료 (%s)\n", resp.Duration.Round(time.Millisecond))
	resp.Provider = cfg.JudgeProvider + " (judge)"
	return resp
}

// consensusReached checks if all responses are substantially similar.
// REQ-7: Uses configurable threshold from OrchestraConfig (default 0.66).
// @AX:NOTE [AUTO] REQ-7 magic constant 0.66 — default consensus threshold; configurable via ConsensusThreshold field
func consensusReached(responses []ProviderResponse, cfg OrchestraConfig) bool {
	if len(responses) < 2 {
		return false
	}
	threshold := cfg.ConsensusThreshold
	if threshold <= 0 {
		threshold = 0.66 // Default consensus threshold
	}
	_, summary := MergeConsensus(responses, threshold)
	n := countNonEmpty(responses)
	return summary == fmt.Sprintf("합의율: %d/%d (100%%)", n, n)
}

// countNonEmpty counts responses with non-empty output.
func countNonEmpty(responses []ProviderResponse) int {
	n := 0
	for _, r := range responses {
		if r.Output != "" {
			n++
		}
	}
	return n
}

// perRoundTimeout calculates the timeout for each debate round.
// REQ-5: Enforces a 45-second minimum floor per round.
// R1: Subtracts judge budget (min 60s) from total before dividing among debate rounds.
// When noJudge is true, the judge budget is skipped entirely so all time goes to debate.
// @AX:NOTE [AUTO] REQ-5 magic constant 45s — minimum floor per debate round; lowering risks premature timeout
func perRoundTimeout(totalSeconds, rounds int, noJudge bool) time.Duration {
	if totalSeconds <= 0 {
		totalSeconds = 120
	}
	if rounds <= 0 {
		rounds = 1
	}
	// Reserve judge budget (min 60s) from total before dividing among debate rounds.
	// Skip reservation when --no-judge is set — no judge phase will run.
	judgeReserve := 60
	if noJudge {
		judgeReserve = 0
	}
	debateBudget := totalSeconds - judgeReserve
	if debateBudget < 0 {
		debateBudget = 0
	}
	perRound := debateBudget / rounds
	if perRound < 45 {
		perRound = 45
	}
	return time.Duration(perRound) * time.Second
}

// buildDebateResult constructs the final OrchestraResult from debate rounds.
func buildDebateResult(cfg OrchestraConfig, responses []ProviderResponse, roundHistory [][]ProviderResponse, start time.Time) *OrchestraResult {
	merged, summary := mergeByStrategy(cfg.Strategy, responses, cfg)
	if merged == "" {
		merged = fmt.Sprintf("[interactive debate] %d rounds completed", len(roundHistory))
	}
	return &OrchestraResult{
		Strategy:     cfg.Strategy,
		Responses:    responses,
		Merged:       merged,
		Duration:     time.Since(start),
		Summary:      summary,
		RoundHistory: roundHistory,
	}
}

// mergeByStrategyWithRoundHistory creates an OrchestraResult from round history.
func mergeByStrategyWithRoundHistory(rounds [][]ProviderResponse, cfg OrchestraConfig) *OrchestraResult {
	var finalResponses []ProviderResponse
	if len(rounds) > 0 {
		finalResponses = rounds[len(rounds)-1]
	}
	merged, summary := mergeByStrategy(cfg.Strategy, finalResponses, cfg)
	return &OrchestraResult{
		Strategy:     cfg.Strategy,
		Responses:    finalResponses,
		Merged:       merged,
		Summary:      summary,
		RoundHistory: rounds,
	}
}
