package telemetry

import (
	"strings"
	"time"
)

// Lead-time event payloads embed the record and add the owning SPEC so that
// cross-process readers can scope events when several runs interleave.
type milestoneEvent struct {
	SpecID string `json:"spec_id,omitempty"`
	Milestone
}

type actionEvent struct {
	SpecID string `json:"spec_id,omitempty"`
	ActionRecord
}

type defectEvent struct {
	SpecID string `json:"spec_id,omitempty"`
	DefectRecord
}

type gateEvent struct {
	SpecID string `json:"spec_id,omitempty"`
	GateRecord
}

type estimateEvent struct {
	SpecID string `json:"spec_id,omitempty"`
	EstimateRecord
}

// RecordMilestone records a named milestone at the current time.
func (r *Recorder) RecordMilestone(name string) Milestone {
	milestone := Milestone{Name: name, At: time.Now()}
	r.mu.Lock()
	r.milestones = append(r.milestones, milestone)
	specID := r.specID
	r.mu.Unlock()

	_ = r.writeEvent(EventTypeMilestone, milestoneEvent{SpecID: specID, Milestone: milestone})
	return milestone
}

// RecordAction records a reread or rerun with the reason it was needed.
func (r *Recorder) RecordAction(kind, target, reason string) ActionRecord {
	action := ActionRecord{Kind: kind, Target: target, Reason: reason, At: time.Now()}
	r.mu.Lock()
	r.actions = append(r.actions, action)
	specID := r.specID
	r.mu.Unlock()

	_ = r.writeEvent(EventTypeAction, actionEvent{SpecID: specID, ActionRecord: action})
	return action
}

// RecordDefect records a defect; At defaults to the current time.
func (r *Recorder) RecordDefect(defect DefectRecord) DefectRecord {
	if defect.At.IsZero() {
		defect.At = time.Now()
	}
	r.mu.Lock()
	r.defects = append(r.defects, defect)
	specID := r.specID
	r.mu.Unlock()

	_ = r.writeEvent(EventTypeDefect, defectEvent{SpecID: specID, DefectRecord: defect})
	return defect
}

// RecordGate records a gate applicability decision; At defaults to now.
func (r *Recorder) RecordGate(gate GateRecord) GateRecord {
	if gate.At.IsZero() {
		gate.At = time.Now()
	}
	r.mu.Lock()
	r.gates = append(r.gates, gate)
	specID := r.specID
	r.mu.Unlock()

	_ = r.writeEvent(EventTypeGate, gateEvent{SpecID: specID, GateRecord: gate})
	return gate
}

// RecordEstimate records the planned lead-time range; a later call replaces
// the earlier one.
func (r *Recorder) RecordEstimate(min, max time.Duration) EstimateRecord {
	estimate := EstimateRecord{Min: min, Max: max}
	r.mu.Lock()
	r.estimate = &estimate
	specID := r.specID
	r.mu.Unlock()

	_ = r.writeEvent(EventTypeEstimate, estimateEvent{SpecID: specID, EstimateRecord: estimate})
	return estimate
}

// cloneDependencies trims, de-duplicates, and copies dependency names so the
// recorded phase never aliases caller-owned memory. Empty input yields nil so
// the field is omitted from JSON.
func cloneDependencies(names []string) []string {
	var deps []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || containsString(deps, name) {
			continue
		}
		deps = append(deps, name)
	}
	return deps
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
