package orchestra

import "strings"

const OrchestrationReceiptSchema = "orchestration_run_receipt.v1"

const (
	TerminalCompleted = "completed"
	TerminalBlocked   = "blocked"
	TerminalSkipped   = "skipped"
	TerminalYielded   = "yielded"
)

const (
	JudgeNotRequested = "not_requested"
	JudgeSkipped      = "skipped"
	JudgePassed       = "passed"
	JudgeFailed       = "failed"
)

func finalizeOrchestraResultForConfig(result *OrchestraResult, cfg OrchestraConfig) *OrchestraResult {
	if result == nil {
		return nil
	}
	aggregateOrchestraUsage(result)
	result.RequestedStrategy = cfg.Strategy
	result.EffectiveStrategy = result.Strategy
	resolved := providerConfigNames(cfg.Providers)
	result.ResolvedProviders = cloneProviderNames(resolved)
	result.RequestedProviders = firstProviderSet(cfg.RequestedProviders, cfg.ConfiguredProviders, resolved)
	result.ConfiguredProviders = firstProviderSet(cfg.ConfiguredProviders, cfg.RequestedProviders, resolved)
	result.QuorumRequired = cfg.MinimumProviders
	if result.RunID == "" {
		result.RunID = ensureRunID(&cfg)
	}
	if cfg.Strategy == StrategyDebate && cfg.JudgeProvider != "" {
		result.JudgeSeparation = evaluateJudgeFamilySeparation(
			cfg.Providers, findOrBuildJudgeConfig(cfg), cfg.RequireJudgeFamilySeparation && !cfg.NoJudge,
		)
	}
	if cfg.Strategy == StrategyConsensus {
		result.ConsensusMetrics = deriveConsensusMetrics(result.Responses, cfg.ConsensusThreshold)
	}
	if result.JudgeStatus == "" && cfg.Strategy == StrategyDebate && cfg.JudgeProvider != "" && cfg.NoJudge {
		result.JudgeStatus = JudgeSkipped
	}
	return finalizeOrchestrationContract(result)
}

// FinalizeOrchestrationResult applies the additive receipt projection to
// execution paths implemented by sibling packages such as structured SPEC
// review while preserving OrchestraResult's legacy fields.
func FinalizeOrchestrationResult(result *OrchestraResult, cfg OrchestraConfig) *OrchestraResult {
	return finalizeOrchestraResultForConfig(result, cfg)
}

func finalizeOrchestrationContract(result *OrchestraResult) *OrchestraResult {
	if result == nil {
		return nil
	}
	result.ReceiptSchema = OrchestrationReceiptSchema
	if result.RequestedStrategy == "" {
		result.RequestedStrategy = result.Strategy
	}
	if result.EffectiveStrategy == "" {
		result.EffectiveStrategy = result.Strategy
	}
	applyResponseTransitionEvidence(result)
	result.AttemptedProviders = firstProviderSet(result.AttemptedProviders, attemptedProviderNames(result))
	result.UsableProviders = usableProviderNames(result)
	result.FailedProviderNames = failedProviderNames(result.FailedProviders)
	if result.DispatchCount == 0 {
		result.DispatchCount = inferredDispatchCount(result)
	}
	if len(result.ConfiguredProviders) == 0 {
		result.ConfiguredProviders = unionProviderNames(result.ResolvedProviders, result.AttemptedProviders)
	}
	if len(result.RequestedProviders) == 0 {
		result.RequestedProviders = cloneProviderNames(result.ConfiguredProviders)
	}
	if len(result.ResolvedProviders) == 0 {
		result.ResolvedProviders = unionProviderNames(result.AttemptedProviders, result.UsableProviders)
	}
	applyProviderIntegrity(result, result.QuorumRequired)
	if result.ConsensusMetrics != nil {
		result.Veto = result.ConsensusMetrics.CriticalVeto
	}
	if result.Veto {
		result.Degraded = true
		result.TerminalState = TerminalBlocked
		appendDegradedReason(result, "critical_dissent_veto")
	}
	if result.JudgeStatus == "" {
		result.JudgeStatus = inferJudgeStatus(result)
	}
	if result.TerminalState == "" {
		result.TerminalState = inferTerminalState(result)
	}
	normalizeGateStatus(result)
	if result.AnalysisVerdict == "" {
		result.AnalysisVerdict = inferAnalysisVerdict(result)
	}
	refreshOrchestrationRunReceipt(result)
	return result
}

func inferJudgeStatus(result *OrchestraResult) string {
	for _, failed := range result.FailedProviders {
		if failed.Role == "judge" {
			return JudgeFailed
		}
	}
	for _, response := range result.Responses {
		if strings.HasSuffix(response.Provider, " (judge)") {
			return JudgePassed
		}
	}
	return JudgeNotRequested
}

func inferTerminalState(result *OrchestraResult) string {
	if result.Yield != nil {
		return TerminalYielded
	}
	if result.JudgeStatus == JudgeFailed || (result.DispatchCount > 0 && len(result.UsableProviders) == 0) {
		return TerminalBlocked
	}
	return TerminalCompleted
}

func inferGateStatus(result *OrchestraResult) string {
	if result.TerminalState == TerminalBlocked {
		return "blocked"
	}
	if result.Degraded {
		return "degraded"
	}
	return "passed"
}

func normalizeGateStatus(result *OrchestraResult) {
	switch result.TerminalState {
	case TerminalBlocked:
		result.GateStatus = "blocked"
	case TerminalSkipped:
		result.GateStatus = "skipped"
	case TerminalYielded:
		result.GateStatus = "yielded"
	default:
		if result.Degraded && (result.GateStatus == "" || result.GateStatus == "passed") {
			result.GateStatus = "degraded"
		} else if result.GateStatus == "" {
			result.GateStatus = inferGateStatus(result)
		}
	}
}

func inferAnalysisVerdict(result *OrchestraResult) string {
	if result.TerminalState == TerminalBlocked {
		return "fail"
	}
	return "pass"
}

func appendDegradedReason(result *OrchestraResult, reason string) {
	for _, existing := range result.DegradedReasons {
		if existing == reason {
			return
		}
	}
	result.DegradedReasons = append(result.DegradedReasons, reason)
}
