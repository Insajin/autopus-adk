package orchestra

import "fmt"

// QuorumSummary renders the quorum decision inputs exactly as decided:
// "usable <usable>/<configured>, required <required>".
func (r *OrchestraResult) QuorumSummary() string {
	return fmt.Sprintf("usable %d/%d, required %d", r.QuorumUsable, len(r.ConfiguredProviders), r.QuorumRequired)
}

// applyProviderIntegrity decides quorum from the configured-provider
// intersection and stores that numerator on the result so every message and
// receipt reports the same count the decision used.
func applyProviderIntegrity(result *OrchestraResult, minimum int) {
	result.QuorumRequired = strategyQuorum(result.EffectiveStrategy, len(result.ConfiguredProviders))
	if minimum > result.QuorumRequired {
		result.QuorumRequired = minimum
	}
	result.QuorumUsable = countConfiguredUsable(result.ConfiguredProviders, result.UsableProviders)
	result.QuorumMet = result.QuorumUsable >= result.QuorumRequired
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
