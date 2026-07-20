package orchestra

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOrchestra_PipelineMiddleFailure_PreservesPartialDispatchReceipt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell failure fixture is unix-specific")
	}
	first := echoProvider("first")
	second := ProviderConfig{
		Name: "second", Binary: "sh", Args: []string{"-c", "exit 23"}, PromptViaArgs: true,
	}

	result, err := RunOrchestra(context.Background(), OrchestraConfig{
		Providers: []ProviderConfig{first, second}, Strategy: StrategyPipeline,
		Prompt: "pipeline contract", TimeoutSeconds: 5,
	})

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Len(t, result.Responses, 1)
	assert.Equal(t, "first", result.Responses[0].Provider)
	assert.Equal(t, 2, result.DispatchCount)
	require.NotNil(t, result.RunReceipt)
	assert.Len(t, result.RunReceipt.ProviderReceipts, 2)
}

func TestRunSubprocessPipeline_RoundTwoFailure_PreservesRoundOneReceipt(t *testing.T) {
	backend := &receiptEvidenceBackend{failRole: "debater_r2", failName: "*"}

	result, err := RunSubprocessPipeline(context.Background(), receiptContractPipeline(backend))

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Len(t, result.RoundHistory, 1)
	assert.Equal(t, 6, result.DispatchCount)
	require.NotNil(t, result.RunReceipt)
	assert.Len(t, result.RunReceipt.ProviderReceipts, 6)
	assert.Equal(t, 6, backend.callCount())
}
