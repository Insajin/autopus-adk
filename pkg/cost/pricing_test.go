package cost_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
)

func TestDefaultPricingTable_ContainsAllModels(t *testing.T) {
	table := cost.DefaultPricingTable()

	required := []string{"claude-opus-4-8", "claude-opus-4-7", "claude-sonnet-5", "claude-sonnet-4-6", "claude-haiku-4-5"}
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
		{"claude-opus-4-8", 5.0, 25.0},
		{"claude-opus-4-7", 5.0, 25.0},
		{"claude-sonnet-5", 3.0, 15.0},
		{"claude-sonnet-4-6", 3.0, 15.0},
		{"claude-haiku-4-5", 1.0, 5.0},
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
		if model, ok := agents[agent]; !ok || model != "claude-opus-4-8" {
			t.Errorf("ultra/%s: want claude-opus-4-8, got %q", agent, model)
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
		{"planner", "claude-opus-4-8"},
		{"architect", "claude-opus-4-8"},
		{"executor", "claude-sonnet-5"},
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
		{"ultra", "executor", "claude-opus-4-8"},
		{"balanced", "executor", "claude-sonnet-5"},
		{"balanced", "validator", "claude-sonnet-5"},
		{"balanced", "planner", "claude-opus-4-8"},
	}

	for _, tc := range cases {
		got := cost.ModelForAgent(tc.mode, tc.agent)
		if got != tc.want {
			t.Errorf("ModelForAgent(%q, %q): want %q, got %q", tc.mode, tc.agent, tc.want, got)
		}
	}
}

// TestModelForAgent_TeamPhaseRoles verifies S3 acceptance values for the three
// team-phase roles added in SPEC-HARNESS-WORKFLOW-TEAM-001 T8.
func TestModelForAgent_TeamPhaseRoles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mode  string
		agent string
		want  string
	}{
		// Existing roles — regression guard (S3 anchor values).
		{"ultra", "executor", "claude-opus-4-8"},
		{"balanced", "executor", "claude-sonnet-5"},
		{"balanced", "planner", "claude-opus-4-8"},
		// New team-phase roles — ultra mode.
		{"ultra", "annotator", "claude-opus-4-8"},
		{"ultra", "security_auditor", "claude-opus-4-8"},
		{"ultra", "test_scaffold", "claude-opus-4-8"},
		// New team-phase roles — balanced mode.
		{"balanced", "annotator", "claude-sonnet-5"},
		{"balanced", "security_auditor", "claude-sonnet-5"},
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
