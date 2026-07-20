package orchestra

import "time"

func activeProviderResults(results []ProviderResult) ([]ProviderResult, bool) {
	active := make([]ProviderResult, 0, len(results))
	hadSkipped := false
	for _, result := range results {
		if result.Response.TerminalState == TerminalSkipped {
			hadSkipped = true
			continue
		}
		active = append(active, result)
	}
	return active, hadSkipped
}

func failedPolicyTransition(failed []FailedProvider) (string, []string, bool) {
	var reasons []string
	terminal := ""
	for _, failure := range failed {
		if !failure.PreflightFailed {
			continue
		}
		for _, reason := range failure.DegradedReasons {
			reasons = appendUniqueName(reasons, reason)
		}
		if failure.TerminalState == TerminalBlocked {
			terminal = TerminalBlocked
		} else if terminal == "" && failure.TerminalState == TerminalSkipped {
			terminal = TerminalSkipped
		}
	}
	return terminal, reasons, terminal != ""
}

func skippedTransitionReasons(responses []ProviderResponse) []string {
	var reasons []string
	for _, response := range responses {
		if response.TerminalState != TerminalSkipped {
			continue
		}
		for _, reason := range response.DegradedReasons {
			reasons = appendUniqueName(reasons, reason)
		}
	}
	return reasons
}

func buildSubprocessPolicyTransition(
	cfg SubprocessPipelineConfig,
	responses []ProviderResponse,
	failed []FailedProvider,
	roundHistory [][]ProviderResponse,
	start time.Time,
	terminal string,
	reasons []string,
) *OrchestraResult {
	judgeStatus := JudgeSkipped
	gateStatus := "skipped"
	verdict := "skipped"
	summary := "subprocess pipeline skipped by pane fallback policy"
	if terminal == TerminalBlocked {
		gateStatus = "blocked"
		verdict = "incomplete"
		summary = "subprocess pipeline blocked by pane fallback policy"
	}
	return finalizeOrchestraResultForConfig(&OrchestraResult{
		Strategy: StrategyDebate, Responses: responses, RoundHistory: roundHistory,
		Duration: time.Since(start), Summary: summary,
		FailedProviders: failed, Degraded: true, DegradedReasons: reasons,
		JudgeStatus: judgeStatus, GateStatus: gateStatus,
		TerminalState: terminal, AnalysisVerdict: verdict,
	}, subprocessPipelineContractConfig(cfg))
}
