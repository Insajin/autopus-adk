package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestOrchestrationRunReceipt_Success_EmitsCommonProviderAndWorkerFields(t *testing.T) {
	t.Parallel()

	backend := &FakeBackend{Responses: []string{
		"plan", "tests", "implementation", "VERDICT: PASS", "VERDICT: APPROVE",
	}}
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID: "SPEC-RECEIPT-001", Platform: "codex",
		Strategy: pipeline.StrategySequential, Backend: backend,
	})

	result, err := engine.Run(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	receipt := result.Receipt
	assert.Equal(t, []string{"codex"}, receipt.RequestedProviders)
	assert.Equal(t, []string{"codex"}, receipt.ConfiguredProviders)
	assert.Equal(t, []string{"codex"}, receipt.ResolvedProviders)
	assert.Equal(t, []string{"codex"}, receipt.AttemptedProviders)
	assert.Equal(t, []string{"codex"}, receipt.UsableProviders)
	assert.Empty(t, receipt.FailedProviders)
	assert.NotNil(t, receipt.WorkerReceipts)
	assert.NotNil(t, receipt.DegradedReasons)
	assert.NotNil(t, receipt.Artifacts)
	require.Len(t, receipt.ProviderReceipts, 5)
	for _, providerReceipt := range receipt.ProviderReceipts {
		assert.Equal(t, "codex", providerReceipt.Provider)
		assert.NotEmpty(t, providerReceipt.Role)
		assert.NotEmpty(t, providerReceipt.Backend)
		assert.True(t, providerReceipt.Usable)
	}
	data, marshalErr := json.Marshal(receipt)
	require.NoError(t, marshalErr)
	for _, key := range []string{
		"provider_receipts", "worker_receipts", "requested_providers", "configured_providers",
		"resolved_providers", "attempted_providers", "usable_providers", "failed_providers",
		"degraded_reasons", "artifacts", "attempts", "judge_status",
	} {
		assert.Contains(t, string(data), `"`+key+`"`)
	}
}

type failingReceiptBackend struct{}

func (failingReceiptBackend) Execute(_ context.Context, req pipeline.PhaseRequest) (*pipeline.PhaseResponse, error) {
	return &pipeline.PhaseResponse{
		Provider: "codex", Backend: "subprocess", Role: string(req.PhaseID),
		FailureClass: "transport_error",
	}, errors.New("transport failed")
}

func TestOrchestrationRunReceipt_BackendFailure_RecordsFailureEvidence(t *testing.T) {
	t.Parallel()

	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID: "SPEC-RECEIPT-002", Platform: "codex",
		Strategy: pipeline.StrategySequential, Backend: failingReceiptBackend{},
	})

	result, err := engine.Run(context.Background())

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, pipeline.TerminalBlocked, result.Receipt.Terminal)
	assert.Equal(t, []string{"codex"}, result.Receipt.FailedProviders)
	assert.Contains(t, result.Receipt.DegradedReasons, "provider_failure")
	require.Len(t, result.Receipt.ProviderReceipts, 1)
	assert.Equal(t, "transport_error", result.Receipt.ProviderReceipts[0].FailureClass)
	assert.False(t, result.Receipt.ProviderReceipts[0].Usable)
}

func TestNewBlockedRunReceipt_RecordsTerminalTransitionAndStableEmptyArrays(t *testing.T) {
	t.Parallel()

	receipt := pipeline.NewBlockedRunReceipt("SPEC-BLOCKED-001", pipeline.StrategySequential, "missing SPEC")

	assert.Equal(t, pipeline.StrategySequential, receipt.EffectiveStrategy)
	assert.NotNil(t, receipt.ProviderReceipts)
	assert.NotNil(t, receipt.WorkerReceipts)
	assert.NotNil(t, receipt.Artifacts)
	assert.NotNil(t, receipt.DegradedReasons)
	require.Len(t, receipt.Transitions, 1)
	assert.Equal(t, string(pipeline.TerminalBlocked), receipt.Transitions[0].State)
}

func TestNewBlockedRunReceipt_UnsupportedStrategyDoesNotInventEffectiveStrategy(t *testing.T) {
	t.Parallel()

	for _, requested := range []pipeline.Strategy{pipeline.StrategyParallel, pipeline.Strategy("future")} {
		receipt := pipeline.NewBlockedRunReceipt("SPEC-BLOCKED-002", requested, "unsupported strategy")

		assert.Equal(t, requested, receipt.RequestedStrategy)
		assert.Empty(t, receipt.EffectiveStrategy)
	}
}
