package cost_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

// TestEstimateCost_ModelNotInCustomPricing verifies that EstimateCost returns
// 0.0 when a model is mapped by QualityModeToModels but is absent from the
// custom pricing table passed to NewEstimatorWithPricing.
func TestEstimateCost_ModelNotInCustomPricing_ReturnsZero(t *testing.T) {
	// Build a pricing table that intentionally omits "claude-opus-4-7".
	partialPricing := map[string]cost.ModelPricing{
		"claude-sonnet-4-6": {InputPricePerMillion: 3.0, OutputPricePerMillion: 15.0},
		"claude-haiku-4-5":  {InputPricePerMillion: 1.0, OutputPricePerMillion: 5.0},
	}
	// ultra/executor → claude-opus-4-7, which is not in partialPricing.
	e := cost.NewEstimatorWithPricing("ultra", partialPricing)
	run := telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 5_000}

	got := e.EstimateCost(run)
	assert.Equal(t, 0.0, got, "model absent from custom pricing table must return 0.0")
}

// TestEstimateCost_StrictQualityMode exercises the "strict" quality mode if it
// maps agents; otherwise verifies graceful zero return.
func TestEstimateCost_StrictMode_ReturnsZeroForUnmappedMode(t *testing.T) {
	// "strict" is not a recognised quality mode in QualityModeToModels.
	e := cost.NewEstimator("strict")
	run := telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 1_000}

	got := e.EstimateCost(run)
	assert.Equal(t, 0.0, got, "unmapped quality mode must return 0.0")
}

// TestEstimatePipelineCost_PhaseWithNoAgents_ReturnsZero verifies that a
// pipeline whose phases contain no agent runs produces a zero total cost.
func TestEstimatePipelineCost_PhaseWithNoAgents_ReturnsZero(t *testing.T) {
	e := cost.NewEstimator("ultra")
	pipeline := telemetry.PipelineRun{
		QualityMode: "ultra",
		Phases: []telemetry.PhaseRecord{
			{Name: "Planning", Agents: nil},
			{Name: "Implementation", Agents: []telemetry.AgentRun{}},
		},
	}

	got := e.EstimatePipelineCost(pipeline)
	assert.Equal(t, 0.0, got)
}

// TestEstimateQualityComparison_LargeTokenCount verifies numeric stability with
// a large (1 million) token count and confirms that ultra remains more expensive.
func TestEstimateQualityComparison_LargeTokenCount_Stable(t *testing.T) {
	e := cost.NewEstimator("balanced")
	ultraCost, balancedCost := e.EstimateQualityComparison(1_000_000)

	assert.Greater(t, ultraCost, 0.0)
	assert.Greater(t, balancedCost, 0.0)
	assert.Greater(t, ultraCost, balancedCost,
		"ultra must remain more expensive than balanced at 1M tokens")
}
