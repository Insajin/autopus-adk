package cost_test

import (
	"math"
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// roundTo6 rounds a float64 to 6 decimal places for stable comparisons.
func roundTo6(v float64) float64 {
	return math.Round(v*1_000_000) / 1_000_000
}

func TestNewEstimator_DefaultPricing(t *testing.T) {
	e := cost.NewEstimator("ultra")
	if e == nil {
		t.Fatal("NewEstimator returned nil")
	}
}

func TestNewEstimatorWithPricing_CustomTable(t *testing.T) {
	custom := map[string]cost.ModelPricing{
		"claude-opus-4": {InputPricePerMillion: 10.0, OutputPricePerMillion: 50.0},
	}
	e := cost.NewEstimatorWithPricing("ultra", custom)
	if e == nil {
		t.Fatal("NewEstimatorWithPricing returned nil")
	}
}

func TestEstimateCost_UltraExecutor(t *testing.T) {
	// ultra/executor → claude-opus-4: input=$15/M, output=$75/M
	// total=4000 → input=3000, output=1000
	// cost = (3000/1_000_000 * 15) + (1000/1_000_000 * 75) = 0.045 + 0.075 = 0.12
	e := cost.NewEstimator("ultra")
	run := telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 4_000}

	got := roundTo6(e.EstimateCost(run))
	want := roundTo6(0.12)
	if got != want {
		t.Errorf("EstimateCost ultra/executor: want %f, got %f", want, got)
	}
}

func TestEstimateCost_BalancedExecutor(t *testing.T) {
	// balanced/executor → claude-sonnet-4: input=$3/M, output=$15/M
	// total=4000 → input=3000, output=1000
	// cost = (3000/1_000_000 * 3) + (1000/1_000_000 * 15) = 0.009 + 0.015 = 0.024
	e := cost.NewEstimator("balanced")
	run := telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 4_000}

	got := roundTo6(e.EstimateCost(run))
	want := roundTo6(0.024)
	if got != want {
		t.Errorf("EstimateCost balanced/executor: want %f, got %f", want, got)
	}
}

func TestEstimateCost_BalancedValidator(t *testing.T) {
	// balanced/validator → claude-sonnet-4: input=$3/M, output=$15/M
	// total=1_000_000 → input=750_000, output=250_000
	// cost = (750_000/1_000_000 * 3.0) + (250_000/1_000_000 * 15.0) = 2.25 + 3.75 = 6.00
	e := cost.NewEstimator("balanced")
	run := telemetry.AgentRun{AgentName: "validator", EstimatedTokens: 1_000_000}

	got := roundTo6(e.EstimateCost(run))
	want := roundTo6(6.00)
	if got != want {
		t.Errorf("EstimateCost balanced/validator: want %f, got %f", want, got)
	}
}

func TestEstimateCost_UnknownMode(t *testing.T) {
	e := cost.NewEstimator("unknown-mode")
	run := telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 1_000}

	got := e.EstimateCost(run)
	if got != 0.0 {
		t.Errorf("unknown mode: want 0.0, got %f", got)
	}
}

func TestEstimateCost_UnknownAgent(t *testing.T) {
	e := cost.NewEstimator("ultra")
	run := telemetry.AgentRun{AgentName: "nonexistent-agent", EstimatedTokens: 1_000}

	got := e.EstimateCost(run)
	if got != 0.0 {
		t.Errorf("unknown agent: want 0.0, got %f", got)
	}
}

func TestEstimateCost_ZeroTokens(t *testing.T) {
	e := cost.NewEstimator("ultra")
	run := telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 0}

	got := e.EstimateCost(run)
	if got != 0.0 {
		t.Errorf("zero tokens: want 0.0, got %f", got)
	}
}

func TestEstimateCost_ImplementsCostEstimatorInterface(t *testing.T) {
	// Verify that *Estimator satisfies telemetry.CostEstimator at compile time.
	var _ telemetry.CostEstimator = cost.NewEstimator("ultra")
}

func TestEstimatePipelineCost_MultiplePhases(t *testing.T) {
	// pipeline QualityMode="balanced"
	// phase1: executor(4000 tokens) + validator(1000 tokens)
	// phase2: planner(2000 tokens)
	// balanced/executor (sonnet): (3000/1M*3)+(1000/1M*15) = 0.000009+0.000015 = 0.000024
	// balanced/validator (sonnet): (750/1M*3)+(250/1M*15) = 0.00000225+0.00000375 = 0.000006
	// balanced/planner (opus):   (1500/1M*15)+(500/1M*75) = 0.0000225+0.0000375 = 0.00006
	e := cost.NewEstimator("ultra") // estimator mode doesn't matter; pipeline overrides it

	pipeline := telemetry.PipelineRun{
		QualityMode: "balanced",
		Phases: []telemetry.PhaseRecord{
			{
				Agents: []telemetry.AgentRun{
					{AgentName: "executor", EstimatedTokens: 4_000},
					{AgentName: "validator", EstimatedTokens: 1_000},
				},
			},
			{
				Agents: []telemetry.AgentRun{
					{AgentName: "planner", EstimatedTokens: 2_000},
				},
			},
		},
	}

	got := e.EstimatePipelineCost(pipeline)
	if got <= 0 {
		t.Errorf("EstimatePipelineCost: expected positive cost, got %f", got)
	}

	// Verify component costs sum correctly using the single-agent helper.
	balancedEst := cost.NewEstimator("balanced")
	wantTotal := balancedEst.EstimateCost(telemetry.AgentRun{AgentName: "executor", EstimatedTokens: 4_000}) +
		balancedEst.EstimateCost(telemetry.AgentRun{AgentName: "validator", EstimatedTokens: 1_000}) +
		balancedEst.EstimateCost(telemetry.AgentRun{AgentName: "planner", EstimatedTokens: 2_000})

	if roundTo6(got) != roundTo6(wantTotal) {
		t.Errorf("EstimatePipelineCost: want %f, got %f", wantTotal, got)
	}
}

func TestEstimatePipelineCost_EmptyPipeline(t *testing.T) {
	e := cost.NewEstimator("balanced")
	pipeline := telemetry.PipelineRun{QualityMode: "balanced", Phases: nil}

	got := e.EstimatePipelineCost(pipeline)
	if got != 0.0 {
		t.Errorf("empty pipeline: want 0.0, got %f", got)
	}
}

func TestEstimateQualityComparison_UltraMoreExpensive(t *testing.T) {
	e := cost.NewEstimator("balanced") // base mode doesn't affect comparison
	ultraCost, balancedCost := e.EstimateQualityComparison(100_000)

	if ultraCost <= 0 {
		t.Errorf("ultra cost should be positive, got %f", ultraCost)
	}
	if balancedCost <= 0 {
		t.Errorf("balanced cost should be positive, got %f", balancedCost)
	}
	if ultraCost <= balancedCost {
		t.Errorf("ultra should cost more than balanced: ultra=%f, balanced=%f", ultraCost, balancedCost)
	}
}

func TestEstimateQualityComparison_ZeroTokens(t *testing.T) {
	e := cost.NewEstimator("ultra")
	ultraCost, balancedCost := e.EstimateQualityComparison(0)

	if ultraCost != 0.0 || balancedCost != 0.0 {
		t.Errorf("zero tokens: want (0,0), got (%f, %f)", ultraCost, balancedCost)
	}
}
