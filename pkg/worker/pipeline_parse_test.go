package worker

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
)

func TestParsePhasePlan(t *testing.T) {
	plan, err := ParsePhasePlan([]string{"planner", "reviewer"})
	if err != nil {
		t.Fatalf("ParsePhasePlan returned error: %v", err)
	}
	if got, want := len(plan), 2; got != want {
		t.Fatalf("len(plan) = %d, want %d", got, want)
	}
	if plan[0] != PhasePlanner || plan[1] != PhaseReviewer {
		t.Fatalf("unexpected phase plan: %v", plan)
	}
}

func TestParsePhasePlan_Invalid(t *testing.T) {
	_, err := ParsePhasePlan([]string{"planner", "deployer"})
	if err == nil {
		t.Fatal("expected invalid phase plan to fail")
	}
}

func TestParsePhaseInstructions(t *testing.T) {
	instructions, err := ParsePhaseInstructions(map[string]string{
		"planner":  "Plan the work carefully.",
		"reviewer": "Review the result rigorously.",
	})
	if err != nil {
		t.Fatalf("ParsePhaseInstructions returned error: %v", err)
	}
	if got, want := len(instructions), 2; got != want {
		t.Fatalf("len(instructions) = %d, want %d", got, want)
	}
	if instructions[PhasePlanner] != "Plan the work carefully." {
		t.Fatalf("unexpected planner instruction: %q", instructions[PhasePlanner])
	}
}

func TestParsePhaseInstructions_Invalid(t *testing.T) {
	_, err := ParsePhaseInstructions(map[string]string{"deployer": "ship it"})
	if err == nil {
		t.Fatal("expected invalid phase instructions to fail")
	}
}

func TestParsePhasePromptTemplates(t *testing.T) {
	templates, err := ParsePhasePromptTemplates(map[string]string{
		"planner":  "PLAN\n\n{{input}}",
		"reviewer": "REVIEW\n\n{{input}}",
	})
	if err != nil {
		t.Fatalf("ParsePhasePromptTemplates returned error: %v", err)
	}
	if got, want := len(templates), 2; got != want {
		t.Fatalf("len(templates) = %d, want %d", got, want)
	}
	if templates[PhasePlanner] != "PLAN\n\n{{input}}" {
		t.Fatalf("unexpected planner template: %q", templates[PhasePlanner])
	}
}

func TestParsePhasePromptTemplates_Invalid(t *testing.T) {
	_, err := ParsePhasePromptTemplates(map[string]string{"deployer": "ship it"})
	if err == nil {
		t.Fatal("expected invalid phase prompt templates to fail")
	}
}

func TestIsContextOverflow(t *testing.T) {
	tests := []struct {
		name string
		evt  adapter.StreamEvent
		want bool
	}{
		{"context window error", adapter.StreamEvent{Type: "error", Data: []byte(`{"error":"context window exceeded"}`)}, true},
		{"token limit error", adapter.StreamEvent{Type: "error", Data: []byte(`{"error":"Token limit reached"}`)}, true},
		{"other error", adapter.StreamEvent{Type: "error", Data: []byte(`{"error":"network timeout"}`)}, false},
		{"non-error event", adapter.StreamEvent{Type: "result", Data: []byte(`{"output":"ok"}`)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsContextOverflow(tt.evt); got != tt.want {
				t.Errorf("IsContextOverflow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPipelineExecutor_SetIterationBudget(t *testing.T) {
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", "/tmp")
	pe.SetIterationBudget(budget.IterationBudget{
		Limit:           20,
		WarnThreshold:   0.70,
		DangerThreshold: 0.90,
	})

	if pe.iterationBudget == nil {
		t.Fatal("iteration budget should be configured")
	}
	if pe.allocator == nil {
		t.Fatal("phase allocator should be configured from iteration budget")
	}
	if got := pe.allocator.PhaseLimit("planner"); got != 2 {
		t.Fatalf("planner phase limit = %d, want 2", got)
	}
}
