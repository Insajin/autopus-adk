package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyAgreementFloor_BlocksBelowFloor(t *testing.T) {
	t.Parallel()
	result := &OrchestraResult{
		ConsensusMetrics: &ConsensusMetrics{TotalClaims: 5, AgreedClaims: 2, AgreementRatio: 0.4},
	}
	applyAgreementFloor(result, 0.9)

	assert.True(t, result.Degraded)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Contains(t, result.DegradedReasons, "agreement_below_floor")
}

// The floor is inclusive: a run that exactly meets the caller's bar ships.
func TestApplyAgreementFloor_AllowsExactlyAtFloor(t *testing.T) {
	t.Parallel()
	result := &OrchestraResult{
		ConsensusMetrics: &ConsensusMetrics{TotalClaims: 4, AgreedClaims: 3, AgreementRatio: 0.75},
	}
	applyAgreementFloor(result, 0.75)

	assert.False(t, result.Degraded)
	assert.Empty(t, result.TerminalState)
	assert.Empty(t, result.DegradedReasons)
}

// Every existing caller passes no floor, so total disagreement must stay a
// non-event unless the gate was explicitly asked for.
func TestApplyAgreementFloor_DisabledWhenUnset(t *testing.T) {
	t.Parallel()
	result := &OrchestraResult{
		ConsensusMetrics: &ConsensusMetrics{TotalClaims: 3, AgreedClaims: 0, AgreementRatio: 0},
	}
	applyAgreementFloor(result, 0)

	assert.False(t, result.Degraded)
	assert.Empty(t, result.TerminalState)
}

func consensusResultForOutputs(outputs ...string) *OrchestraResult {
	responses := make([]ProviderResponse, 0, len(outputs))
	for i, output := range outputs {
		responses = append(responses, ProviderResponse{
			Provider: string(rune('a' + i)), Output: output, ExitCode: 0,
		})
	}
	return &OrchestraResult{Strategy: StrategyConsensus, Responses: responses, Merged: "merged"}
}

func consensusCfgWithFloor(providers int, floor float64) OrchestraConfig {
	cfgProviders := make([]ProviderConfig, 0, providers)
	for i := range providers {
		cfgProviders = append(cfgProviders, ProviderConfig{Name: string(rune('a' + i))})
	}
	return OrchestraConfig{
		Strategy: StrategyConsensus, Providers: cfgProviders, MinimumAgreementRatio: floor,
	}
}

// Providers that share no claims must stop at the gate rather than hand back a
// merged answer neither of them corroborated.
func TestFinalizeConsensus_FloorBlocksDisagreeingProviders(t *testing.T) {
	t.Parallel()
	result := finalizeOrchestraResultForConfig(
		consensusResultForOutputs("alpha finding one", "beta finding two"),
		consensusCfgWithFloor(2, 0.9),
	)
	require.NotNil(t, result)
	require.NotNil(t, result.ConsensusMetrics)

	assert.Less(t, result.ConsensusMetrics.AgreementRatio, 0.9)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Equal(t, "blocked", result.GateStatus)
	assert.Contains(t, result.DegradedReasons, "agreement_below_floor")
	// The blocked run is exactly when the caller needs evidence, so the receipt
	// must still be projected.
	require.NotNil(t, result.RunReceipt)
	assert.Equal(t, OrchestrationReceiptSchema, result.RunReceipt.Schema)
}

func TestFinalizeConsensus_FloorPassesWhenProvidersAgree(t *testing.T) {
	t.Parallel()
	result := finalizeOrchestraResultForConfig(
		consensusResultForOutputs("shared finding", "shared finding"),
		consensusCfgWithFloor(2, 0.9),
	)
	require.NotNil(t, result)
	require.NotNil(t, result.ConsensusMetrics)

	assert.GreaterOrEqual(t, result.ConsensusMetrics.AgreementRatio, 0.9)
	assert.Equal(t, TerminalCompleted, result.TerminalState)
	assert.Equal(t, "passed", result.GateStatus)
	assert.NotContains(t, result.DegradedReasons, "agreement_below_floor")
}

// Same disagreeing providers, no floor: the historical behaviour is untouched.
func TestFinalizeConsensus_NoFloorLeavesDisagreementUnblocked(t *testing.T) {
	t.Parallel()
	result := finalizeOrchestraResultForConfig(
		consensusResultForOutputs("alpha finding one", "beta finding two"),
		consensusCfgWithFloor(2, 0),
	)
	require.NotNil(t, result)
	assert.Equal(t, TerminalCompleted, result.TerminalState)
	assert.NotContains(t, result.DegradedReasons, "agreement_below_floor")
}
