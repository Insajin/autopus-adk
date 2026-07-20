package orchestra

import (
	"time"
)

func buildSubprocessParticipantFailure(
	cfg SubprocessPipelineConfig,
	failed []FailedProvider,
	roundHistory [][]ProviderResponse,
	start time.Time,
	runErr error,
) *OrchestraResult {
	var responses []ProviderResponse
	if len(roundHistory) > 0 {
		responses = append(responses, roundHistory[len(roundHistory)-1]...)
	}
	result := &OrchestraResult{
		Strategy: StrategyDebate, Responses: responses, RoundHistory: roundHistory,
		Duration: time.Since(start), Summary: "subprocess pipeline blocked before required judge",
		FailedProviders: failed, Degraded: true,
		DegradedReasons: []string{"participant_or_stage_failure"},
		JudgeStatus:     JudgeFailed, GateStatus: "blocked",
		TerminalState: TerminalBlocked, AnalysisVerdict: "incomplete",
	}
	if runErr != nil {
		result.Summary += ": " + runErr.Error()
	}
	return finalizeOrchestraResultForConfig(result, subprocessPipelineContractConfig(cfg))
}
