package budget

import "testing"

// REQ-BUDGET-09: Pipeline phase budget distribution.
func TestPhaseAllocator_PhaseLimits(t *testing.T) {
	pa := NewPhaseAllocator(100, DefaultAllocation())

	tests := []struct {
		phase string
		want  int
	}{
		{"planner", 10},
		{"executor", 60},
		{"tester", 20},
		{"reviewer", 10},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			got := pa.PhaseLimit(tt.phase)
			if got != tt.want {
				t.Errorf("PhaseLimit(%q) = %d, want %d", tt.phase, got, tt.want)
			}
		})
	}
}

func TestPhaseAllocator_UnknownPhase(t *testing.T) {
	pa := NewPhaseAllocator(100, DefaultAllocation())
	if got := pa.PhaseLimit("unknown"); got != 0 {
		t.Errorf("PhaseLimit(unknown) = %d, want 0", got)
	}
}

// REQ-BUDGET-10: Unused budget carry-over.
func TestPhaseAllocator_CarryOver(t *testing.T) {
	pa := NewPhaseAllocator(100, DefaultAllocation())

	// Planner uses 5 of its 10 budget -> 5 carry over.
	pa.CompletePhase("planner", 5)

	// Executor should get 60 (base) + 5 (carry) = 65.
	got := pa.PhaseLimit("executor")
	if got != 65 {
		t.Errorf("executor limit with carry = %d, want 65", got)
	}
}

func TestPhaseAllocator_CarryOverChain(t *testing.T) {
	pa := NewPhaseAllocator(100, DefaultAllocation())

	// Planner: limit=10, used=3 -> carry=7.
	pa.CompletePhase("planner", 3)

	// Executor: limit=60+7=67, used=50 -> carry=17.
	pa.CompletePhase("executor", 50)

	// Tester: limit=20+17=37.
	got := pa.PhaseLimit("tester")
	if got != 37 {
		t.Errorf("tester limit with chained carry = %d, want 37", got)
	}
}

func TestPhaseAllocator_OveruseNoNegativeCarry(t *testing.T) {
	pa := NewPhaseAllocator(100, DefaultAllocation())

	// Planner uses more than its limit.
	pa.CompletePhase("planner", 15)

	// Carry should be 0, not negative.
	got := pa.PhaseLimit("executor")
	if got != 60 {
		t.Errorf("executor limit after overuse = %d, want 60", got)
	}
}

func TestPhaseAllocator_TotalUsedAndRemaining(t *testing.T) {
	pa := NewPhaseAllocator(100, DefaultAllocation())

	pa.CompletePhase("planner", 8)
	pa.CompletePhase("executor", 55)

	if pa.TotalUsed() != 63 {
		t.Errorf("TotalUsed = %d, want 63", pa.TotalUsed())
	}
	if pa.TotalRemaining() != 37 {
		t.Errorf("TotalRemaining = %d, want 37", pa.TotalRemaining())
	}
}
