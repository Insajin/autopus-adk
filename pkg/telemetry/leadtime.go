package telemetry

import (
	"fmt"
	"time"
)

// Safety gate statuses reported per mandatory gate.
const (
	SafetyGateResolved      = "resolved"
	SafetyGateUnresolved    = "unresolved"
	SafetyGateBlocked       = "blocked"
	SafetyGateNotApplicable = "not_applicable"
	SafetyGateOmitted       = "omitted"
)

// PhaseDefects counts defects discovered in one phase and the files they touched.
type PhaseDefects struct {
	Count        int `json:"count"`
	FilesTouched int `json:"files_touched"`
}

// EstimateVsActual compares the planned lead-time range with completion time.
type EstimateVsActual struct {
	Min         time.Duration `json:"min_ns"`
	Max         time.Duration `json:"max_ns"`
	Actual      time.Duration `json:"actual_ns"`
	WithinRange bool          `json:"within_range"`
}

// LeadTimeReport is the body-free lead-time view of one pipeline run. Every
// duration is serialized as nanoseconds; TimeToFirstSlice is null when the
// first_vertical_slice milestone was never recorded.
type LeadTimeReport struct {
	SpecID                  string                  `json:"spec_id"`
	CompletionLeadTime      time.Duration           `json:"completion_lead_time_ns"`
	TimeToFirstSlice        *time.Duration          `json:"time_to_first_slice_ns"`
	CriticalPath            []string                `json:"critical_path"`
	CriticalPathDuration    time.Duration           `json:"critical_path_duration_ns"`
	RereadCount             int                     `json:"reread_count"`
	RerunCount              int                     `json:"rerun_count"`
	Rereads                 map[string]int          `json:"rereads_by_reason"`
	Reruns                  map[string]int          `json:"reruns_by_reason"`
	DefectsByDiscoveryPhase map[string]PhaseDefects `json:"defects_by_discovery_phase"`
	RepeatFindingRate       float64                 `json:"repeat_finding_rate"`
	EscapedDefects          int                     `json:"escaped_defects"`
	UnresolvedSafetyGates   []string                `json:"unresolved_safety_gates"`
	SafetyGateStatus        map[string]string       `json:"safety_gate_status"`
	EstimateVsActual        *EstimateVsActual       `json:"estimate_vs_actual"`
	Issues                  []string                `json:"issues,omitempty"`
}

// ComputeLeadTime derives lead-time, rework, defect, and safety-gate metrics
// from a run. Inconsistent inputs (unknown dependencies, milestones before the
// run started) are reported in Issues instead of aborting the report.
func ComputeLeadTime(run PipelineRun) LeadTimeReport {
	report := LeadTimeReport{
		SpecID:                  run.SpecID,
		CompletionLeadTime:      completionLeadTime(run),
		Rereads:                 make(map[string]int),
		Reruns:                  make(map[string]int),
		DefectsByDiscoveryPhase: make(map[string]PhaseDefects),
		UnresolvedSafetyGates:   make([]string, 0),
		SafetyGateStatus:        make(map[string]string, len(SafetyGates)),
	}
	report.TimeToFirstSlice = timeToFirstSlice(run, &report.Issues)

	var pathIssues []string
	report.CriticalPath, report.CriticalPathDuration, pathIssues = criticalPath(run.Phases)
	report.Issues = append(report.Issues, pathIssues...)

	for _, action := range run.Actions {
		reason := action.Reason
		if reason == "" {
			reason = "unspecified"
		}
		switch action.Kind {
		case ActionKindReread:
			report.RereadCount++
			report.Rereads[reason]++
		case ActionKindRerun:
			report.RerunCount++
			report.Reruns[reason]++
		default:
			report.Issues = append(report.Issues, fmt.Sprintf("unknown action kind %q ignored", action.Kind))
		}
	}

	defects := latestDefects(run.Defects)
	repeats := 0
	for _, defect := range defects {
		phase := defect.DiscoveredPhase
		if phase == "" {
			phase = "unknown"
		}
		bucket := report.DefectsByDiscoveryPhase[phase]
		bucket.Count++
		bucket.FilesTouched += defect.FilesTouched
		report.DefectsByDiscoveryPhase[phase] = bucket
		if defect.Escaped {
			report.EscapedDefects++
		}
		if defect.Repeat {
			repeats++
		}
	}
	if len(defects) > 0 {
		report.RepeatFindingRate = float64(repeats) / float64(len(defects))
	}

	latestGate := make(map[string]GateRecord, len(run.Gates))
	for _, gate := range run.Gates {
		latestGate[gate.Gate] = gate
	}
	for _, gate := range SafetyGates {
		status := safetyGateStatus(latestGate[gate])
		report.SafetyGateStatus[gate] = status
		if status != SafetyGateResolved {
			report.UnresolvedSafetyGates = append(report.UnresolvedSafetyGates, gate)
		}
	}

	if run.Estimate != nil {
		actual := report.CompletionLeadTime
		report.EstimateVsActual = &EstimateVsActual{
			Min:         run.Estimate.Min,
			Max:         run.Estimate.Max,
			Actual:      actual,
			WithinRange: actual >= run.Estimate.Min && actual <= run.Estimate.Max,
		}
	}
	return report
}

func completionLeadTime(run PipelineRun) time.Duration {
	if run.TotalDuration > 0 {
		return run.TotalDuration
	}
	if !run.StartTime.IsZero() && !run.EndTime.IsZero() && run.EndTime.After(run.StartTime) {
		return run.EndTime.Sub(run.StartTime)
	}
	return 0
}

// timeToFirstSlice measures the earliest first_vertical_slice milestone
// against the run start; nil when the milestone is absent or unmeasurable.
func timeToFirstSlice(run PipelineRun, issues *[]string) *time.Duration {
	var earliest *time.Time
	for i := range run.Milestones {
		milestone := run.Milestones[i]
		if milestone.Name != MilestoneFirstVerticalSlice {
			continue
		}
		if earliest == nil || milestone.At.Before(*earliest) {
			at := milestone.At
			earliest = &at
		}
	}
	if earliest == nil {
		return nil
	}
	if run.StartTime.IsZero() {
		*issues = append(*issues, "first_vertical_slice recorded but run start time is unknown")
		return nil
	}
	elapsed := earliest.Sub(run.StartTime)
	if elapsed < 0 {
		*issues = append(*issues, "first_vertical_slice precedes run start")
		return nil
	}
	return &elapsed
}

// latestDefects keeps the last record per defect ID (a defect may be recorded
// again once fixed) in first-seen order; records without an ID stay distinct.
func latestDefects(defects []DefectRecord) []DefectRecord {
	out := make([]DefectRecord, 0, len(defects))
	position := make(map[string]int, len(defects))
	for _, defect := range defects {
		if defect.ID == "" {
			out = append(out, defect)
			continue
		}
		if index, seen := position[defect.ID]; seen {
			out[index] = defect
			continue
		}
		position[defect.ID] = len(out)
		out = append(out, defect)
	}
	return out
}

// safetyGateStatus classifies the latest record of a mandatory gate. A zero
// record means the gate was never recorded, which is an omission.
func safetyGateStatus(gate GateRecord) string {
	switch {
	case gate.Gate == "":
		return SafetyGateOmitted
	case gate.Applicability == GateApplicabilityBlocked:
		return SafetyGateBlocked
	case gate.Applicability == GateApplicabilityNotApplicable:
		return SafetyGateNotApplicable
	case gate.Resolved:
		return SafetyGateResolved
	default:
		return SafetyGateUnresolved
	}
}
