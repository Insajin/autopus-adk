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
		err := session.WaitForDoneRound(timeout, p.Name, round)
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
// Interactive pane-based judge execution is unreliable because the pane already contains
// previous round output, making prompt-based completion detection ambiguous.
func runJudgeRound(ctx context.Context, cfg OrchestraConfig, _ []paneInfo, _ *HookSession, responses []ProviderResponse, _ int) *ProviderResponse {
	judgment := buildJudgmentPrompt(cfg.Prompt, responses)
	judgeCfg := findOrBuildJudgeConfig(cfg)

	// Run judge as non-interactive process for reliable result collection.
	resp, err := runProvider(ctx, judgeCfg, judgment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Judge] 프로세스 실행 실패: %v\n", err)
		return nil
	}
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
// @AX:NOTE [AUTO] REQ-5 magic constant 45s — minimum floor per debate round; lowering risks premature timeout
func perRoundTimeout(totalSeconds, rounds int) time.Duration {
	if totalSeconds <= 0 {
		totalSeconds = 120
	}
	if rounds <= 0 {
		rounds = 1
	}
	perRound := totalSeconds / rounds
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
