package cost_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
)

func TestDefaultPricingTable_ContainsAllModels(t *testing.T) {
	table := cost.DefaultPricingTable()

	required := []string{"claude-opus-5", "claude-opus-4-8", "claude-opus-4-7", "claude-sonnet-5", "claude-sonnet-4-6", "claude-haiku-4-5", "claude-fable-5"}
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
		{"claude-opus-5", 5.0, 25.0},
		{"claude-opus-4-8", 5.0, 25.0},
		{"claude-opus-4-7", 5.0, 25.0},
		{"claude-sonnet-5", 3.0, 15.0},
		{"claude-sonnet-4-6", 3.0, 15.0},
		{"claude-haiku-4-5", 1.0, 5.0},
		{"claude-fable-5", 10.0, 50.0},
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

func TestDefaultPricingTable_FableAliasesRemainUnpriced(t *testing.T) {
	t.Parallel()

	table := cost.DefaultPricingTable()
	for _, alias := range []string{"fable", "best"} {
		if _, ok := table[alias]; ok {
			t.Errorf("dynamic alias %q must not have deterministic pricing", alias)
		}
	}
}

func TestQualityModeToModels_FableIsNotDefault(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"ultra", "balanced"} {
		for agent, model := range cost.QualityModeToModels(mode) {
			if model == "claude-fable-5" || model == "fable" || model == "best" {
				t.Errorf("%s/%s unexpectedly defaults to Fable model %q", mode, agent, model)
			}
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
		if model, ok := agents[agent]; !ok || model != "claude-opus-5" {
			t.Errorf("ultra/%s: want claude-opus-5, got %q", agent, model)
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
		{"planner", "claude-opus-5"},
		{"architect", "claude-opus-5"},
		// executor and security-auditor sit at opus in the balanced preset,
		// which is now the only tier source for cost.
		{"executor", "claude-opus-5"},
		{"tester", "claude-sonnet-5"},
		{"reviewer", "claude-sonnet-5"},
		{"validator", "claude-sonnet-5"},
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
		{"ultra", "executor", "claude-opus-5"},
		{"balanced", "executor", "claude-opus-5"},
		{"balanced", "validator", "claude-sonnet-5"},
		{"balanced", "planner", "claude-opus-5"},
	}

	for _, tc := range cases {
		got := cost.ModelForAgent(tc.mode, tc.agent)
		if got != tc.want {
			t.Errorf("ModelForAgent(%q, %q): want %q, got %q", tc.mode, tc.agent, tc.want, got)
		}
	}
}

// TestModelForAgent_TeamPhaseRoles covers the workflow phase roles added in
// SPEC-HARNESS-WORKFLOW-TEAM-001 T8. The S3 tier shape still holds (ultra is
// uniformly opus); the balanced values now follow the config quality preset,
// which promoted executor and security-auditor to opus.
func TestModelForAgent_TeamPhaseRoles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mode  string
		agent string
		want  string
	}{
		// Core roles — regression guard.
		{"ultra", "executor", "claude-opus-5"},
		{"balanced", "executor", "claude-opus-5"},
		{"balanced", "planner", "claude-opus-5"},
		// New team-phase roles — ultra mode.
		{"ultra", "annotator", "claude-opus-5"},
		{"ultra", "security_auditor", "claude-opus-5"},
		{"ultra", "test_scaffold", "claude-opus-5"},
		// New team-phase roles — balanced mode.
		{"balanced", "annotator", "claude-sonnet-5"},
		{"balanced", "security_auditor", "claude-opus-5"},
		{"balanced", "test_scaffold", "claude-sonnet-5"},
	}

	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(tc.mode+"/"+tc.agent, func(t *testing.T) {
			t.Parallel()
			got := cost.ModelForAgent(tc.mode, tc.agent)
			if got == "" {
				t.Errorf("ModelForAgent(%q, %q): got empty string, want %q", tc.mode, tc.agent, tc.want)
			}
			if got != tc.want {
				t.Errorf("ModelForAgent(%q, %q): want %q, got %q", tc.mode, tc.agent, tc.want, got)
			}
		})
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
