package orchestra

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJudgeProviderSelection_InvokingProviderMatchProjectsToResultAndReceipt(t *testing.T) {
	t.Parallel()
	result := FinalizeOrchestrationResult(judgeProviderSelectionResult("codex"), OrchestraConfig{
		Strategy:             StrategyDebate,
		Providers:            []ProviderConfig{{Name: "codex"}},
		ConfiguredProviders:  []string{"codex"},
		JudgeProvider:        "Codex",
		InvokingProvider:     "CODEX",
		JudgeSelectionSource: "invoking_provider",
	})

	require.NotNil(t, result.JudgeProviderSelection)
	assert.Equal(t, "codex", result.JudgeProviderSelection.InvokingProvider)
	assert.Equal(t, "codex", result.JudgeProviderSelection.SelectedJudgeProvider)
	assert.Equal(t, "invoking_provider", result.JudgeProviderSelection.SelectionSource)
	require.NotNil(t, result.JudgeProviderSelection.ProviderMatch)
	assert.True(t, *result.JudgeProviderSelection.ProviderMatch)
	require.NotNil(t, result.RunReceipt)
	assert.Equal(t, result.JudgeProviderSelection, result.RunReceipt.JudgeProviderSelection)

	wire, err := json.Marshal(result.RunReceipt)
	require.NoError(t, err)
	assert.Contains(t, string(wire), `"judge_provider_selection"`)
	selectionWire, err := json.Marshal(result.RunReceipt.JudgeProviderSelection)
	require.NoError(t, err)
	assert.NotContains(t, string(selectionWire), "CODEX")
	assert.NotContains(t, string(selectionWire), "Codex")
}

func TestJudgeProviderSelection_ExplicitMismatchIsObservableWithoutChangingGate(t *testing.T) {
	t.Parallel()
	result := FinalizeOrchestrationResult(judgeProviderSelectionResult("gemini"), OrchestraConfig{
		Strategy:             StrategyDebate,
		Providers:            []ProviderConfig{{Name: "codex"}, {Name: "gemini"}},
		ConfiguredProviders:  []string{"codex"},
		JudgeProvider:        "gemini",
		InvokingProvider:     "codex",
		JudgeSelectionSource: "explicit",
	})

	require.NotNil(t, result.JudgeProviderSelection)
	assert.Equal(t, "codex", result.JudgeProviderSelection.InvokingProvider)
	assert.Equal(t, "gemini", result.JudgeProviderSelection.SelectedJudgeProvider)
	assert.Equal(t, "explicit", result.JudgeProviderSelection.SelectionSource)
	require.NotNil(t, result.JudgeProviderSelection.ProviderMatch)
	assert.False(t, *result.JudgeProviderSelection.ProviderMatch)
	assert.Equal(t, JudgePassed, result.JudgeStatus)
	assert.True(t, result.QuorumMet)
	assert.Equal(t, TerminalCompleted, result.TerminalState)
	assert.Equal(t, "passed", result.GateStatus)
	assert.False(t, result.Degraded)
	assert.Empty(t, result.DegradedReasons)
	require.NotNil(t, result.RunReceipt)
	assert.Equal(t, result.JudgeProviderSelection, result.RunReceipt.JudgeProviderSelection)
	assert.Equal(t, "passed", result.RunReceipt.GateStatus)
}

func judgeProviderSelectionResult(judge string) *OrchestraResult {
	return &OrchestraResult{
		Strategy: StrategyDebate,
		Responses: []ProviderResponse{
			{
				Provider: "codex", Output: "usable participant answer",
				Role: "debater_r2", Attempt: 2, ExecutedBackend: "pane", TerminalState: TerminalCompleted,
			},
			{
				Provider: judge + " (judge)", Output: `{"recommendation":"proceed"}`,
				Role: "judge", Attempt: 3, ExecutedBackend: "pane", TerminalState: TerminalCompleted,
			},
		},
	}
}
