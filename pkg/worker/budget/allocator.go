package budget

// PhaseAllocation maps pipeline phases to their budget percentages.
// Default: Planning 10%, Execution 60%, Testing 20%, Review 10%.
type PhaseAllocation struct {
	Planning  float64
	Execution float64
	Testing   float64
	Review    float64
}

// DefaultAllocation returns the standard phase budget split.
func DefaultAllocation() PhaseAllocation {
	return PhaseAllocation{
		Planning:  0.10,
		Execution: 0.60,
		Testing:   0.20,
		Review:    0.10,
	}
}

// PhaseAllocator distributes a total budget across pipeline phases
// and carries over unused budget from completed phases.
type PhaseAllocator struct {
	total      int
	allocation PhaseAllocation
	used       int // total tool calls used across all phases
	carryOver  int // unused budget carried from previous phases
}

// NewPhaseAllocator creates an allocator with the given total budget.
func NewPhaseAllocator(total int, alloc PhaseAllocation) *PhaseAllocator {
	return &PhaseAllocator{
		total:      total,
		allocation: alloc,
	}
}

// PhaseLimit returns the budget limit for a given phase name,
// including any carry-over from previous phases.
func (pa *PhaseAllocator) PhaseLimit(phase string) int {
	pct := pa.phasePct(phase)
	base := int(float64(pa.total) * pct)
	return base + pa.carryOver
}

// CompletePhase records how many tool calls were used in the phase
// and calculates carry-over for the next phase.
func (pa *PhaseAllocator) CompletePhase(phase string, usedInPhase int) {
	limit := pa.PhaseLimit(phase)
	pa.used += usedInPhase

	pa.carryOver = max(limit-usedInPhase, 0)
}

// TotalUsed returns the total tool calls used across all completed phases.
func (pa *PhaseAllocator) TotalUsed() int {
	return pa.used
}

// TotalRemaining returns how many tool calls remain of the total budget.
func (pa *PhaseAllocator) TotalRemaining() int {
	rem := pa.total - pa.used
	if rem < 0 {
		return 0
	}
	return rem
}

func (pa *PhaseAllocator) phasePct(phase string) float64 {
	switch phase {
	case "planner":
		return pa.allocation.Planning
	case "executor":
		return pa.allocation.Execution
	case "tester":
		return pa.allocation.Testing
	case "reviewer":
		return pa.allocation.Review
	default:
		return 0
	}
}
