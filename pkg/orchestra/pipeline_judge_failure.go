package orchestra

import "time"

func buildSubprocessJudgeFailure(
	cfg SubprocessPipelineConfig,
	failed []FailedProvider,
	roundHistory [][]ProviderResponse,
	r1Results []ProviderResult,
	r2Results []ProviderResult,
	start time.Time,
	judgeResponse *ProviderResponse,
	runErr error,
) *OrchestraResult {
	finalResults := r2Results
	if len(finalResults) == 0 {
		finalResults = r1Results
	}
	responses := providerResultsToResponses(finalResults)
	judgeFailure := buildFailedProviderWithContext(
		cfg.Judge, judgeResponse, runErr, cfg.TimeoutSeconds, "judge", len(responses) > 0,
	)
	judgeFailure.Attempt = cfg.Rounds + 2
	judgeFailure.ModelFamily = cfg.Judge.ModelFamily
	judgeFailure.ExecutedBackend = cfg.Backend.Name()
	if judgeResponse != nil && judgeResponse.ExecutedBackend != "" {
		judgeFailure.ExecutedBackend = judgeResponse.ExecutedBackend
	}
	failed = append(failed, judgeFailure)
	result := &OrchestraResult{
		Strategy:        StrategyDebate,
		Responses:       responses,
		Duration:        time.Since(start),
		Summary:         "subprocess pipeline blocked: required judge failed",
		FailedProviders: failed,
		RoundHistory:    roundHistory,
		Degraded:        true,
		DegradedReasons: []string{"judge_failure"},
		JudgeStatus:     JudgeFailed,
		GateStatus:      "blocked",
		TerminalState:   TerminalBlocked,
		AnalysisVerdict: "incomplete",
	}
	return finalizeOrchestraResultForConfig(result, subprocessPipelineContractConfig(cfg))
}
