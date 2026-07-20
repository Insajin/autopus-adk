package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderIntegrity_FastestUsesSingleWinnerQuorum(t *testing.T) {
	t.Parallel()

	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy:           StrategyFastest,
		Responses:          []ProviderResponse{{Provider: "codex", Output: "winner"}},
		AttemptedProviders: []string{"claude", "codex", "gemini"},
		DispatchCount:      3,
	}, OrchestraConfig{
		Strategy:            StrategyFastest,
		Providers:           []ProviderConfig{{Name: "claude"}, {Name: "codex"}, {Name: "gemini"}},
		ConfiguredProviders: []string{"claude", "codex", "gemini"},
	})

	assert.Equal(t, 1, result.QuorumRequired)
	assert.True(t, result.QuorumMet)
	assert.Equal(t, []string{"claude", "codex", "gemini"}, result.AttemptedProviders)
	assert.Equal(t, []string{"codex"}, result.UsableProviders)
	assert.Empty(t, result.FailedProviderNames)
	assert.Equal(t, 3, result.DispatchCount)
	assert.Equal(t, "passed", result.GateStatus)
	assert.Equal(t, TerminalCompleted, result.TerminalState)
	assert.NotContains(t, result.DegradedReasons, "provider_quorum")
	assert.Len(t, result.RunReceipt.ProviderReceipts, 3)
	cancelled := 0
	for _, receipt := range result.RunReceipt.ProviderReceipts {
		if receipt.FailureClass == "strategy_cancelled" {
			cancelled++
		}
	}
	assert.Equal(t, 2, cancelled)
}
