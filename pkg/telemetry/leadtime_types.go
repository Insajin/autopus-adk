package telemetry

import (
	"fmt"
	"time"
)

// MilestoneFirstVerticalSlice is the milestone name that marks the first
// end-to-end working slice; time-to-first-slice is measured against it.
const MilestoneFirstVerticalSlice = "first_vertical_slice"

// Action kinds recorded for rework accounting.
const (
	ActionKindReread = "reread"
	ActionKindRerun  = "rerun"
)

// Gate applicability values shared with the gate applicability receipt.
const (
	GateApplicabilityRequired      = "required"
	GateApplicabilityReusable      = "reusable"
	GateApplicabilityNotApplicable = "not_applicable"
	GateApplicabilityBlocked       = "blocked"
)

// SafetyGates are the mandatory gates that may never be omitted or declared
// not applicable. Their order is the order reported.
var SafetyGates = []string{"security", "validation", "data_loss", "deterministic_oracle"}

// Milestone marks a named moment inside a pipeline run.
type Milestone struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
}

// ActionRecord records rework: a re-read of a path or a re-run of a command.
type ActionRecord struct {
	Kind   string    `json:"kind"` // reread or rerun
	Target string    `json:"target"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// DefectRecord records where a defect was discovered and fixed. Escaped marks
// a defect found after the run should have caught it; Repeat marks a finding
// already reported on identical inputs.
type DefectRecord struct {
	ID              string    `json:"id"`
	DiscoveredPhase string    `json:"discovered_phase"`
	FixedPhase      string    `json:"fixed_phase,omitempty"`
	FilesTouched    int       `json:"files_touched"`
	Escaped         bool      `json:"escaped"`
	Repeat          bool      `json:"repeat"`
	At              time.Time `json:"at"`
}

// GateRecord records one applicability decision for a gate and whether the
// gate was resolved (evidence produced or reused).
type GateRecord struct {
	Gate          string    `json:"gate"`
	Applicability string    `json:"applicability"`
	Resolved      bool      `json:"resolved"`
	At            time.Time `json:"at"`
}

// EstimateRecord is the planned lead-time range for the run.
type EstimateRecord struct {
	Min time.Duration `json:"min_ns"`
	Max time.Duration `json:"max_ns"`
}

// ValidateActionKind rejects action kinds outside reread|rerun.
func ValidateActionKind(kind string) error {
	switch kind {
	case ActionKindReread, ActionKindRerun:
		return nil
	default:
		return fmt.Errorf("unknown action kind %q (want: %s|%s)", kind, ActionKindReread, ActionKindRerun)
	}
}

// ValidateGateApplicability rejects applicability values outside the closed
// set and refuses to declare a mandatory safety gate not applicable.
func ValidateGateApplicability(gate, applicability string) error {
	switch applicability {
	case GateApplicabilityRequired, GateApplicabilityReusable, GateApplicabilityBlocked:
		return nil
	case GateApplicabilityNotApplicable:
		if IsSafetyGate(gate) {
			return fmt.Errorf("safety gate %q cannot be %s", gate, GateApplicabilityNotApplicable)
		}
		return nil
	default:
		return fmt.Errorf("unknown gate applicability %q (want: %s|%s|%s|%s)", applicability,
			GateApplicabilityRequired, GateApplicabilityReusable, GateApplicabilityNotApplicable, GateApplicabilityBlocked)
	}
}

// ValidateEstimate requires a positive, ordered min..max range.
func ValidateEstimate(min, max time.Duration) error {
	if min <= 0 || max <= 0 {
		return fmt.Errorf("estimate range must be positive (min %s, max %s)", min, max)
	}
	if min > max {
		return fmt.Errorf("estimate min %s exceeds max %s", min, max)
	}
	return nil
}

// IsSafetyGate reports whether gate is one of the mandatory safety gates.
func IsSafetyGate(gate string) bool {
	for _, safety := range SafetyGates {
		if safety == gate {
			return true
		}
	}
	return false
}
