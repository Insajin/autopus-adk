package telemetry

import (
	"fmt"
	"time"
)

// LeadTimeComparison reports how a run moved against a baseline run. The
// regression rule is fail-closed on safety: more escaped defects or more
// unresolved safety gates than the baseline is a regression regardless of
// how much faster the run finished.
type LeadTimeComparison struct {
	BaselineSpecID                string         `json:"baseline_spec_id"`
	DeltaFirstSlice               *time.Duration `json:"delta_first_slice_ns"`
	DeltaCompletion               time.Duration  `json:"delta_completion_ns"`
	DeltaCriticalPath             time.Duration  `json:"delta_critical_path_ns"`
	BaselineEscapedDefects        int            `json:"baseline_escaped_defects"`
	EscapedDefectsDelta           int            `json:"escaped_defects_delta"`
	BaselineUnresolvedSafetyGates int            `json:"baseline_unresolved_safety_gates"`
	UnresolvedSafetyGatesDelta    int            `json:"unresolved_safety_gates_delta"`
	Regression                    bool           `json:"regression"`
	RegressionReasons             []string       `json:"regression_reasons,omitempty"`
}

// CompareLeadTime computes current minus baseline deltas. DeltaFirstSlice is
// nil unless both runs recorded the first_vertical_slice milestone.
func CompareLeadTime(current, baseline LeadTimeReport) LeadTimeComparison {
	comparison := LeadTimeComparison{
		BaselineSpecID:                baseline.SpecID,
		DeltaCompletion:               current.CompletionLeadTime - baseline.CompletionLeadTime,
		DeltaCriticalPath:             current.CriticalPathDuration - baseline.CriticalPathDuration,
		BaselineEscapedDefects:        baseline.EscapedDefects,
		EscapedDefectsDelta:           current.EscapedDefects - baseline.EscapedDefects,
		BaselineUnresolvedSafetyGates: len(baseline.UnresolvedSafetyGates),
		UnresolvedSafetyGatesDelta:    len(current.UnresolvedSafetyGates) - len(baseline.UnresolvedSafetyGates),
	}
	if current.TimeToFirstSlice != nil && baseline.TimeToFirstSlice != nil {
		delta := *current.TimeToFirstSlice - *baseline.TimeToFirstSlice
		comparison.DeltaFirstSlice = &delta
	}
	if comparison.EscapedDefectsDelta > 0 {
		comparison.RegressionReasons = append(comparison.RegressionReasons,
			fmt.Sprintf("escaped_defects increased %d -> %d", baseline.EscapedDefects, current.EscapedDefects))
	}
	if comparison.UnresolvedSafetyGatesDelta > 0 {
		comparison.RegressionReasons = append(comparison.RegressionReasons,
			fmt.Sprintf("unresolved_safety_gates increased %d -> %d", len(baseline.UnresolvedSafetyGates), len(current.UnresolvedSafetyGates)))
	}
	comparison.Regression = len(comparison.RegressionReasons) > 0
	return comparison
}
