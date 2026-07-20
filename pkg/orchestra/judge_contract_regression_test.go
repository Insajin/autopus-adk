package orchestra

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSubprocessPipeline_RequiredJudgeFamilySeparation_FailsClosedBeforeDispatch(t *testing.T) {
	tests := []struct {
		name        string
		participant string
		judge       string
		wantReason  string
	}{
		{name: "same family", participant: "openai", judge: "openai", wantReason: "same_model_family"},
		{name: "unknown judge family", participant: "openai", judge: "", wantReason: "unknown_judge_model_family"},
		{name: "unknown participant family", participant: "", judge: "anthropic", wantReason: "unknown_participant_model_family"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &receiptEvidenceBackend{}
			cfg := receiptContractPipeline(backend)
			for i := range cfg.Providers {
				cfg.Providers[i].ModelFamily = tt.participant
			}
			cfg.Judge.ModelFamily = tt.judge

			result, err := RunSubprocessPipeline(context.Background(), cfg)

			require.Error(t, err)
			require.NotNil(t, result)
			assert.Zero(t, backend.callCount())
			assert.Equal(t, TerminalBlocked, result.TerminalState)
			require.NotNil(t, result.RunReceipt)
			require.NotNil(t, result.RunReceipt.JudgeSeparation)
			assert.True(t, result.RunReceipt.JudgeSeparation.Required)
			assert.False(t, result.RunReceipt.JudgeSeparation.Separated)
			assert.Equal(t, tt.wantReason, result.RunReceipt.JudgeSeparation.Reason)
		})
	}
}

func TestRunOrchestra_RequiredJudgeMalformedOutput_BlocksAndPreservesParticipants(t *testing.T) {
	participant := echoProvider("participant")
	participant.ModelFamily = "participant-family"
	judge := echoProvider("judge")
	judge.ModelFamily = "judge-family"

	result, err := RunOrchestra(context.Background(), OrchestraConfig{
		Providers: []ProviderConfig{participant}, Strategy: StrategyDebate,
		Prompt: "typed judge", TimeoutSeconds: 5, JudgeProvider: judge.Name,
		JudgeConfig: &judge, RequireJudgeFamilySeparation: true,
	})

	require.Error(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Responses)
	assert.Equal(t, "participant", result.Responses[0].Provider)
	assert.Equal(t, JudgeFailed, result.JudgeStatus)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Contains(t, result.DegradedReasons, "judge_failure")
	require.NotNil(t, result.RunReceipt.JudgeSeparation)
	assert.True(t, result.RunReceipt.JudgeSeparation.Separated)
}
