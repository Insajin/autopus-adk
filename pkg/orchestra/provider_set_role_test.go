package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderIntegrity_JudgeSuccessDoesNotSatisfyFailedParticipantQuorum(t *testing.T) {
	t.Parallel()
	participantR1 := ProviderResponse{
		Provider: "claude", Error: "round 1 failed", TimedOut: true,
		Role: "debater_r1", Attempt: 1, ExecutedBackend: "pane", TerminalState: TerminalBlocked,
	}
	participantR2 := ProviderResponse{
		Provider: "claude", Error: "round 2 failed", TimedOut: true,
		Role: "debater_r2", Attempt: 2, ExecutedBackend: "pane", TerminalState: TerminalBlocked,
	}
	judge := ProviderResponse{
		Provider: "claude (judge)", Output: `{"recommendation":"judge succeeded"}`,
		Role: "judge", Attempt: 3, ExecutedBackend: "pane", TerminalState: TerminalCompleted,
	}

	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy:     StrategyDebate,
		Responses:    []ProviderResponse{participantR2, judge},
		RoundHistory: [][]ProviderResponse{{participantR1}, {participantR2}},
		FailedProviders: []FailedProvider{
			{Name: "claude", Role: "debater_r1", Attempt: 1, Error: participantR1.Error},
			{Name: "claude", Role: "debater_r2", Attempt: 2, Error: participantR2.Error},
		},
	}, OrchestraConfig{
		Strategy:            StrategyDebate,
		Providers:           []ProviderConfig{{Name: "claude"}},
		ConfiguredProviders: []string{"claude"},
		JudgeProvider:       "claude",
	})

	require.NotNil(t, result)
	assert.Equal(t, JudgePassed, result.JudgeStatus)
	assert.NotContains(t, result.UsableProviders, "claude")
	assert.False(t, result.QuorumMet)
	assert.Contains(t, result.DegradedReasons, "provider_quorum")
	require.NotNil(t, result.RunReceipt)
	assert.False(t, result.RunReceipt.QuorumMet)
}

func TestProviderIntegrity_JudgeFailureDoesNotEraseUsableParticipantQuorum(t *testing.T) {
	t.Parallel()
	participant := ProviderResponse{
		Provider: "claude", Output: "usable participant answer",
		Role: "debater_r2", Attempt: 2, ExecutedBackend: "pane", TerminalState: TerminalCompleted,
	}

	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy:     StrategyDebate,
		Responses:    []ProviderResponse{participant},
		RoundHistory: [][]ProviderResponse{{participant}},
		FailedProviders: []FailedProvider{{
			Name: "claude", Role: "judge", Attempt: 3, Error: "judge failed",
		}},
	}, OrchestraConfig{
		Strategy:            StrategyDebate,
		Providers:           []ProviderConfig{{Name: "claude"}},
		ConfiguredProviders: []string{"claude"},
		JudgeProvider:       "claude",
	})

	require.NotNil(t, result)
	assert.Equal(t, JudgeFailed, result.JudgeStatus)
	assert.Contains(t, result.UsableProviders, "claude")
	assert.True(t, result.QuorumMet)
	require.NotNil(t, result.RunReceipt)
	assert.True(t, result.RunReceipt.QuorumMet)
}
