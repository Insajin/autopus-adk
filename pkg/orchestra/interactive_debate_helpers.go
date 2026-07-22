package orchestra

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// runJudgeRound uses pane-aware backend selection with a bounded fresh context.
// The parent context remains the outer cancellation bound.
func runJudgeRound(ctx context.Context, cfg OrchestraConfig, _ []paneInfo, _ *HookSession, responses []ProviderResponse, round int) *ProviderResponse {
	judgment := buildTypedJudgmentPrompt(cfg.Prompt, responses)
	judgeCfg := findOrBuildJudgeConfig(cfg)

	judgeTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if judgeTimeout < 60*time.Second {
		judgeTimeout = 60 * time.Second
	}
	judgeCtx, cancel := context.WithTimeout(ctx, judgeTimeout)
	defer cancel()

	backendCfg := cfg
	backendCfg.HookMode = false
	backendCfg.SessionID = ""
	backend := SelectBackend(backendCfg)
	fmt.Fprintf(os.Stderr, "[Judge] %s 실행 중 (provider: %s, timeout: %s)...\n", backend.Name(), cfg.JudgeProvider, judgeTimeout)
	resp, err := backend.Execute(judgeCtx, ProviderRequest{
		Provider: judgeCfg.Name,
		Prompt:   judgment,
		Role:     "judge",
		Round:    round + 1,
		Timeout:  judgeTimeout,
		Config:   judgeCfg,
	})
	if err != nil {
		executedBackend := backend.Name()
		if resp != nil && resp.ExecutedBackend != "" {
			executedBackend = resp.ExecutedBackend
		}
		fmt.Fprintf(os.Stderr, "[Judge] %s 실행 실패: %v\n", executedBackend, err)
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
	if _, parseErr := (&OutputParser{}).ParseJudge(resp.Output); parseErr != nil {
		fmt.Fprintf(os.Stderr, "[Judge] invalid typed output: %v\n", parseErr)
		return nil
	}
	if resp.ExecutedBackend == "" {
		resp.ExecutedBackend = backend.Name()
	}
	fmt.Fprintf(os.Stderr, "[Judge] 판정 완료 (backend=%s, %s)\n", resp.ExecutedBackend, resp.Duration.Round(time.Millisecond))
	resp.Provider = cfg.JudgeProvider + " (judge)"
	return resp
}

// @AX:NOTE [AUTO] REQ-7 magic constant 0.66 — default consensus threshold; configurable via ConsensusThreshold field
func consensusReached(responses []ProviderResponse, cfg OrchestraConfig) bool {
	if len(responses) < 2 {
		return false
	}
	threshold := cfg.ConsensusThreshold
	if threshold <= 0 {
		threshold = 0.66 // Default consensus threshold
	}
	metrics := deriveConsensusMetrics(responses, threshold)
	return metrics != nil && metrics.TotalClaims > 0 && metrics.DissentClaims == 0
}

func countNonEmpty(responses []ProviderResponse) int {
	n := 0
	for _, r := range responses {
		if r.Output != "" {
			n++
		}
	}
	return n
}

// @AX:NOTE [AUTO] REQ-5 magic constant 45s — minimum floor per debate round; lowering risks premature timeout
func perRoundTimeout(totalSeconds, rounds int, noJudge bool) time.Duration {
	if totalSeconds <= 0 {
		totalSeconds = 120
	}
	if rounds <= 0 {
		rounds = 1
	}
	// Reserve the judge budget unless --no-judge is set.
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

func buildDebateResult(cfg OrchestraConfig, responses []ProviderResponse, roundHistory [][]ProviderResponse, start time.Time) *OrchestraResult {
	merged, summary := mergeByStrategy(cfg.Strategy, responses, cfg)
	if merged == "" {
		merged = fmt.Sprintf("[interactive debate] %d rounds completed", len(roundHistory))
	}
	result := &OrchestraResult{
		Strategy:     cfg.Strategy,
		Responses:    responses,
		Merged:       merged,
		Duration:     time.Since(start),
		Summary:      summary,
		RoundHistory: roundHistory,
	}
	result.FailedProviders = deriveFailedProviders(roundHistory)
	result.Degraded = len(result.FailedProviders) > 0
	result.RunID = cfg.RunID
	if cfg.ReliabilityStore != nil {
		bundleSummary := result.Summary
		nextStep := "inspect reliability receipts"
		if result.Degraded {
			bundleSummary = fmt.Sprintf("%s | degraded providers: %s", result.Summary, joinFailedProviderNames(result.FailedProviders))
			nextStep = "retry failed providers with subprocess fallback"
		}
		bundlePath := cfg.ReliabilityStore.writeFailureBundle(bundleSummary, nextStep, result.Degraded)
		result.Reliability = cfg.ReliabilityStore.summary(bundlePath)
	}
	if result.Degraded && !strings.Contains(result.Summary, "degraded") {
		result.Summary = fmt.Sprintf("%s (degraded: %s)", result.Summary, joinFailedProviderNames(result.FailedProviders))
	}
	return finalizeOrchestraResultForConfig(result, cfg)
}

func deriveFailedProviders(roundHistory [][]ProviderResponse) []FailedProvider {
	seen := map[string]FailedProvider{}
	for roundIndex, responses := range roundHistory {
		for _, response := range responses {
			switch {
			case response.TimedOut:
				seen[response.Provider] = FailedProvider{
					Name:             response.Provider,
					Error:            fmt.Sprintf("round %d timeout", roundIndex+1),
					CollectionMode:   "hook",
					Receipt:          response.Receipt,
					CorrelationRunID: "",
					NextRemediation:  "retry with subprocess fallback",
				}
			case response.EmptyOutput || response.Output == "":
				seen[response.Provider] = FailedProvider{
					Name:             response.Provider,
					Error:            fmt.Sprintf("round %d empty output", roundIndex+1),
					CollectionMode:   "hook",
					Receipt:          response.Receipt,
					CorrelationRunID: "",
					NextRemediation:  "inspect collection receipt and retry",
				}
			}
		}
	}
	failed := make([]FailedProvider, 0, len(seen))
	for _, entry := range seen {
		failed = append(failed, entry)
	}
	return failed
}

func joinFailedProviderNames(failed []FailedProvider) string {
	if len(failed) == 0 {
		return ""
	}
	names := make([]string, 0, len(failed))
	for _, entry := range failed {
		names = append(names, entry.Name)
	}
	return strings.Join(names, ", ")
}

// mergeByStrategyWithRoundHistory creates an OrchestraResult from round history.
func mergeByStrategyWithRoundHistory(rounds [][]ProviderResponse, cfg OrchestraConfig) *OrchestraResult {
	var finalResponses []ProviderResponse
	if len(rounds) > 0 {
		finalResponses = rounds[len(rounds)-1]
	}
	merged, summary := mergeByStrategy(cfg.Strategy, finalResponses, cfg)
	return finalizeOrchestraResultForConfig(&OrchestraResult{
		Strategy:     cfg.Strategy,
		Responses:    finalResponses,
		Merged:       merged,
		Summary:      summary,
		RoundHistory: rounds,
	}, cfg)
}
