package budget

import "testing"

// REQ-BUDGET-04: IterationBudget struct with thresholds.

func TestDefaultBudget(t *testing.T) {
	b := DefaultBudget(100)
	if b.Limit != 100 {
		t.Errorf("Limit = %d, want 100", b.Limit)
	}
	if b.WarnThreshold != 0.70 {
		t.Errorf("WarnThreshold = %f, want 0.70", b.WarnThreshold)
	}
	if b.DangerThreshold != 0.90 {
		t.Errorf("DangerThreshold = %f, want 0.90", b.DangerThreshold)
	}
}

func TestEvaluate_Levels(t *testing.T) {
	b := DefaultBudget(100)

	tests := []struct {
		name  string
		count int
		want  ThresholdLevel
	}{
		{"zero", 0, LevelOK},
		{"below warn", 69, LevelOK},
		{"at warn", 70, LevelWarn},
		{"mid warn", 80, LevelWarn},
		{"at danger", 90, LevelDanger},
		{"mid danger", 95, LevelDanger},
		{"at limit", 100, LevelExhausted},
		{"over limit", 150, LevelExhausted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.Evaluate(tt.count)
			if got != tt.want {
				t.Errorf("Evaluate(%d) = %d, want %d", tt.count, got, tt.want)
			}
		})
	}
}

func TestEvaluate_ZeroLimit(t *testing.T) {
	b := IterationBudget{Limit: 0}
	if got := b.Evaluate(5); got != LevelOK {
		t.Errorf("Evaluate with zero limit = %d, want LevelOK", got)
	}
}

func TestBudget_String(t *testing.T) {
	b := DefaultBudget(100)
	s := b.String()
	if s == "" {
		t.Error("String() returned empty")
	}
}
