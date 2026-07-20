package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsensus_MalformedPeerCannotSuppressStructuredCriticalVeto(t *testing.T) {
	t.Parallel()

	responses := []ProviderResponse{
		{Provider: "claude", Output: marshalReviewerOutput(t, []Finding{{
			Severity: "critical", Category: "security", ScopeRef: "pkg/key.go",
			Description: "Rotate the leaked signing key",
		}})},
		{Provider: "codex", Output: "unstructured approval"},
	}
	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy: StrategyConsensus, Responses: responses,
	}, OrchestraConfig{
		Strategy:            StrategyConsensus,
		Providers:           []ProviderConfig{{Name: "claude"}, {Name: "codex"}},
		ConfiguredProviders: []string{"claude", "codex"},
	})

	require.NotNil(t, result.ConsensusMetrics)
	assert.True(t, result.ConsensusMetrics.CriticalVeto)
	assert.True(t, result.Veto)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Equal(t, "blocked", result.GateStatus)
}
