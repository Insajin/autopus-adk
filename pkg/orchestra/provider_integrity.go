package orchestra

func applyProviderIntegrity(result *OrchestraResult, minimum int) {
	result.QuorumRequired = strategyQuorum(result.EffectiveStrategy, len(result.ConfiguredProviders))
	if minimum > result.QuorumRequired {
		result.QuorumRequired = minimum
	}
	result.QuorumMet = countConfiguredUsable(result.ConfiguredProviders, result.UsableProviders) >= result.QuorumRequired
	if len(result.FailedProviders) > 0 {
		result.Degraded = true
		appendDegradedReason(result, "provider_failure")
	}
	if result.QuorumMet {
		return
	}
	result.Degraded = true
	appendDegradedReason(result, "provider_quorum")
	result.GateStatus = "blocked"
}

func strategyQuorum(strategy Strategy, configured int) int {
	if strategy == StrategyFastest {
		return 1
	}
	return majorityQuorum(configured)
}
