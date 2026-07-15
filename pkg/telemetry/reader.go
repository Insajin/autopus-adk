// Package telemetry provides utilities for reading and filtering JSONL telemetry events.
package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// telemetrySubDir is the sub-path appended to baseDir when scanning for JSONL files.
// @AX:NOTE: [AUTO] magic constant — canonical telemetry storage path; change only with migration
const telemetrySubDir = ".autopus/telemetry"

// LoadEvents reads a JSONL file line by line, parses each line into an Event, and returns
// all results. Blank lines are skipped. File-not-found returns an empty slice, not an error.
func LoadEvents(filePath string) ([]Event, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}

// LoadAllEvents merges events from all .jsonl files under {baseDir}/.autopus/telemetry/
// and returns them sorted by timestamp ascending.
// Missing directory returns an empty slice, not an error.
// @AX:ANCHOR: [AUTO] signature must not change — LatestPipelineRun, PipelineRunsBySpecID, and tests depend on it
// @AX:REASON: 3+ callers rely on this contract; coordinate any signature change with all consumers
func LoadAllEvents(baseDir string) ([]Event, error) {
	dir := filepath.Join(baseDir, telemetrySubDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}

	var all []Event
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		events, err := LoadEvents(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Timestamp.Before(all[j].Timestamp) })
	return all, nil
}

// FilterByType returns the subset of events whose Type equals eventType.
func FilterByType(events []Event, eventType string) []Event {
	out := make([]Event, 0)
	for _, e := range events {
		if e.Type == eventType {
			out = append(out, e)
		}
	}
	return out
}

// LatestPipelineRun returns the most recent pipeline_end PipelineRun from baseDir.
// Returns nil (not an error) when no pipeline_end events exist.
func LatestPipelineRun(baseDir string) (*PipelineRun, error) {
	events, err := LoadAllEvents(baseDir)
	if err != nil {
		return nil, err
	}
	runs, err := decodePipelineRuns(events)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[len(runs)-1], nil
}

// PipelineRunsBySpecID returns all pipeline_end PipelineRun values from baseDir
// whose SpecID matches the given specID.
func PipelineRunsBySpecID(baseDir, specID string) ([]PipelineRun, error) {
	events, err := LoadAllEvents(baseDir)
	if err != nil {
		return nil, err
	}
	decoded, err := decodePipelineRuns(events)
	if err != nil {
		return nil, err
	}
	var runs []PipelineRun
	for _, run := range decoded {
		if run.SpecID == specID {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func decodePipelineRuns(events []Event) ([]PipelineRun, error) {
	completed := make([]PipelineRun, 0)
	fallback := make([]PipelineRun, 0)
	for i, event := range events {
		if event.Type != EventTypePipelineEnd {
			continue
		}
		var run PipelineRun
		if err := json.Unmarshal(event.Data, &run); err != nil {
			return nil, err
		}
		if len(run.Phases) == 0 {
			hydratePipelineRun(events, i, event.Timestamp, &run)
		}
		fallback = append(fallback, run)
		if run.FinalStatus != "" {
			completed = append(completed, run)
		}
	}
	if len(completed) > 0 {
		return completed, nil
	}
	return fallback, nil
}

func hydratePipelineRun(events []Event, endIndex int, endTime time.Time, run *PipelineRun) {
	startIndex := latestPipelineStart(events, endIndex, run.SpecID)
	if startIndex < 0 {
		return
	}
	run.StartTime = events[startIndex].Timestamp
	run.EndTime = endTime
	run.TotalDuration = endTime.Sub(run.StartTime)
	var start struct {
		QualityMode string `json:"quality_mode"`
	}
	_ = json.Unmarshal(events[startIndex].Data, &start)
	if run.QualityMode == "" {
		run.QualityMode = start.QualityMode
	}
	phaseIndex := make(map[string]int)
	for _, event := range events[startIndex+1 : endIndex] {
		if event.Type != EventTypeAgentRun {
			continue
		}
		var agent AgentRun
		if json.Unmarshal(event.Data, &agent) != nil || agent.SpecID != run.SpecID {
			continue
		}
		phase := agent.Phase
		if phase == "" {
			phase = "agent"
		}
		index, exists := phaseIndex[phase]
		if !exists {
			index = len(run.Phases)
			phaseIndex[phase] = index
			run.Phases = append(run.Phases, PhaseRecord{Name: phase, StartTime: agent.StartTime})
		}
		record := &run.Phases[index]
		record.Agents = append(record.Agents, agent)
		record.EndTime = agent.EndTime
		record.Duration += agent.Duration
		record.Status = agent.Status
	}
}

func latestPipelineStart(events []Event, endIndex int, specID string) int {
	for i := endIndex - 1; i >= 0; i-- {
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
	return -1
}
