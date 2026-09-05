package telemetry

import "encoding/json"

// hydrateLeadTime rebuilds lead-time records and phase dependencies from the
// events that follow a run's pipeline_start. The JSONL stream is the source of
// truth: every writer emits one event per record, so the snapshot embedded in
// pipeline_end is replaced whenever events exist. The window extends to the
// next pipeline_start of the same SPEC (or the end of the stream) so defects
// that escape and are recorded after pipeline_end still attribute to the run.
func hydrateLeadTime(events []Event, startIndex int, run *PipelineRun) {
	end := nextPipelineStart(events, startIndex, run.SpecID)
	var (
		milestones []Milestone
		actions    []ActionRecord
		defects    []DefectRecord
		gates      []GateRecord
		estimate   *EstimateRecord
	)
	dependencies := make(map[string][]string)
	for _, event := range events[startIndex+1 : end] {
		switch event.Type {
		case EventTypePhaseStart:
			if start, ok := decodeScoped[phaseStartEvent](event, run.SpecID); ok && len(start.DependsOn) > 0 {
				dependencies[start.Name] = cloneDependencies(append(dependencies[start.Name], start.DependsOn...))
			}
		case EventTypeMilestone:
			if record, ok := decodeScoped[Milestone](event, run.SpecID); ok {
				if record.At.IsZero() {
					record.At = event.Timestamp
				}
				milestones = append(milestones, record)
			}
		case EventTypeAction:
			if record, ok := decodeScoped[ActionRecord](event, run.SpecID); ok {
				if record.At.IsZero() {
					record.At = event.Timestamp
				}
				actions = append(actions, record)
			}
		case EventTypeDefect:
			if record, ok := decodeScoped[DefectRecord](event, run.SpecID); ok {
				if record.At.IsZero() {
					record.At = event.Timestamp
				}
				defects = append(defects, record)
			}
		case EventTypeGate:
			if record, ok := decodeScoped[GateRecord](event, run.SpecID); ok {
				if record.At.IsZero() {
					record.At = event.Timestamp
				}
				gates = append(gates, record)
			}
		case EventTypeEstimate:
			if record, ok := decodeScoped[EstimateRecord](event, run.SpecID); ok {
				estimate = &record
			}
		}
	}
	if len(milestones) > 0 {
		run.Milestones = milestones
	}
	if len(actions) > 0 {
		run.Actions = actions
	}
	if len(defects) > 0 {
		run.Defects = defects
	}
	if len(gates) > 0 {
		run.Gates = gates
	}
	if estimate != nil {
		run.Estimate = estimate
	}
	for i := range run.Phases {
		if len(run.Phases[i].DependsOn) > 0 {
			continue
		}
		if deps, ok := dependencies[run.Phases[i].Name]; ok {
			run.Phases[i].DependsOn = deps
		}
	}
}

// decodeScoped decodes an event payload of type T when it belongs to the run:
// payloads name their SPEC in spec_id, and payloads written before SPEC
// scoping (no spec_id) are accepted because the caller already bounded the
// event window to this run.
func decodeScoped[T any](event Event, runSpecID string) (T, bool) {
	var header struct {
		SpecID string `json:"spec_id"`
	}
	var record T
	if json.Unmarshal(event.Data, &header) != nil || !specMatches(header.SpecID, runSpecID) {
		return record, false
	}
	if json.Unmarshal(event.Data, &record) != nil {
		return record, false
	}
	return record, true
}

// nextPipelineStart returns the index of the next pipeline_start for specID
// after startIndex, or len(events) when the run is the last one recorded.
func nextPipelineStart(events []Event, startIndex int, specID string) int {
	for i := startIndex + 1; i < len(events); i++ {
		if events[i].Type != EventTypePipelineStart {
			continue
		}
		var start struct {
			SpecID string `json:"spec_id"`
		}
		if json.Unmarshal(events[i].Data, &start) == nil && start.SpecID == specID {
			return i
		}
	}
	return len(events)
}

// specMatches accepts events that name the run's SPEC or predate SPEC scoping.
func specMatches(eventSpecID, runSpecID string) bool {
	return eventSpecID == "" || eventSpecID == runSpecID
}
