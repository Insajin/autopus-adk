package cost_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
)

func TestDefaultPricingTable_ContainsAllModels(t *testing.T) {
	table := cost.DefaultPricingTable()

	required := []string{"claude-opus-4", "claude-sonnet-4", "claude-haiku-4.5"}
	for _, model := range required {
		if _, ok := table[model]; !ok {
			t.Errorf("pricing table missing model: %s", model)
		}
	}
}

func TestDefaultPricingTable_Prices(t *testing.T) {
	table := cost.DefaultPricingTable()

	cases := []struct {
		model  string
		input  float64
		output float64
	}{
		{"claude-opus-4", 15.0, 75.0},
		{"claude-sonnet-4", 3.0, 15.0},
		{"claude-haiku-4.5", 0.80, 4.0},
	}

	for _, tc := range cases {
		p, ok := table[tc.model]
		if !ok {
			t.Fatalf("model not found: %s", tc.model)
		}
		if p.InputPricePerMillion != tc.input {
			t.Errorf("%s input price: want %.2f, got %.2f", tc.model, tc.input, p.InputPricePerMillion)
		}
		if p.OutputPricePerMillion != tc.output {
			t.Errorf("%s output price: want %.2f, got %.2f", tc.model, tc.output, p.OutputPricePerMillion)
		}
	}
}

func TestQualityModeToModels_Ultra(t *testing.T) {
	agents := cost.QualityModeToModels("ultra")
	if agents == nil {
		t.Fatal("ultra mode returned nil")
	}

	expected := []string{"planner", "architect", "executor", "tester", "reviewer", "validator"}
	for _, agent := range expected {
		if model, ok := agents[agent]; !ok || model != "claude-opus-4" {
			t.Errorf("ultra/%s: want claude-opus-4, got %q", agent, model)
		}
	}
}

func TestQualityModeToModels_Balanced(t *testing.T) {
	agents := cost.QualityModeToModels("balanced")
	if agents == nil {
		t.Fatal("balanced mode returned nil")
	}

	cases := []struct {
		agent string
		model string
	}{
		{"planner", "claude-opus-4"},
		{"architect", "claude-opus-4"},
		{"executor", "claude-sonnet-4"},
		{"tester", "claude-sonnet-4"},
		{"reviewer", "claude-sonnet-4"},
		{"validator", "claude-sonnet-4"},
	}

	for _, tc := range cases {
		if got := agents[tc.agent]; got != tc.model {
			t.Errorf("balanced/%s: want %s, got %s", tc.agent, tc.model, got)
		}
	}
}

func TestQualityModeToModels_Unknown(t *testing.T) {
	if got := cost.QualityModeToModels("nonexistent"); got != nil {
		t.Errorf("unknown mode should return nil, got %v", got)
	}
}

func TestModelForAgent_Known(t *testing.T) {
	cases := []struct {
		mode  string
		agent string
		want  string
	}{
		{"ultra", "executor", "claude-opus-4"},
		{"balanced", "executor", "claude-sonnet-4"},
		{"balanced", "validator", "claude-sonnet-4"},
		{"balanced", "planner", "claude-opus-4"},
	}

	for _, tc := range cases {
		got := cost.ModelForAgent(tc.mode, tc.agent)
		if got != tc.want {
			t.Errorf("ModelForAgent(%q, %q): want %q, got %q", tc.mode, tc.agent, tc.want, got)
		}
	}
}

func TestModelForAgent_Unknown(t *testing.T) {
	if got := cost.ModelForAgent("unknown-mode", "planner"); got != "" {
		t.Errorf("unknown mode should return empty string, got %q", got)
	}

	if got := cost.ModelForAgent("balanced", "nonexistent-agent"); got != "" {
		t.Errorf("unknown agent should return empty string, got %q", got)
	}
}
