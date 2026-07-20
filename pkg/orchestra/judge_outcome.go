package orchestra

import "fmt"

func finalizeDebateOutcome(result *OrchestraResult, cfg OrchestraConfig) (*OrchestraResult, error) {
	result = finalizeOrchestraResultForConfig(result, cfg)
	if result == nil || cfg.Strategy != StrategyDebate || cfg.JudgeProvider == "" || cfg.NoJudge || result.Yield != nil {
		return result, nil
	}
	if result.JudgeStatus == JudgePassed {
		return result, nil
	}
	if !hasJudgeFailure(result.FailedProviders) {
		result.FailedProviders = append(result.FailedProviders, FailedProvider{
			Name:            cfg.JudgeProvider,
			Role:            "judge",
			Error:           "required judge did not produce a usable verdict",
			FailureClass:    "execution_error",
			NextRemediation: "retry the judge or use --no-judge explicitly",
		})
	}
	result.JudgeStatus = JudgeFailed
	result.Degraded = true
	result.TerminalState = TerminalBlocked
	result.GateStatus = "blocked"
	result.AnalysisVerdict = "incomplete"
	appendDegradedReason(result, "judge_failure")
	result = finalizeOrchestraResultForConfig(result, cfg)
	return result, requiredJudgeError(result, cfg.JudgeProvider)
}

func hasJudgeFailure(failed []FailedProvider) bool {
	for _, entry := range failed {
		if entry.Role == "judge" {
			return true
		}
	}
	return false
}

func requiredJudgeError(result *OrchestraResult, judge string) error {
	detail := "required judge did not produce a usable verdict"
	for _, entry := range result.FailedProviders {
		if entry.Role == "judge" && entry.Error != "" {
			detail = entry.Error
			break
		}
	}
	return fmt.Errorf("orchestra: required judge %q failed: %s", judge, detail)
}
