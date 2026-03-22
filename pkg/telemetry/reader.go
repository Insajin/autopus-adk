// Package telemetry provides utilities for reading and filtering JSONL telemetry events.
package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	pipeline := FilterByType(events, EventTypePipelineEnd)
	if len(pipeline) == 0 {
		return nil, nil
	}
	// Events are sorted ascending; last entry is most recent.
	var run PipelineRun
	if err := json.Unmarshal(pipeline[len(pipeline)-1].Data, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// PipelineRunsBySpecID returns all pipeline_end PipelineRun values from baseDir
// whose SpecID matches the given specID.
func PipelineRunsBySpecID(baseDir, specID string) ([]PipelineRun, error) {
	events, err := LoadAllEvents(baseDir)
	if err != nil {
		return nil, err
	}
	var runs []PipelineRun
	for _, e := range FilterByType(events, EventTypePipelineEnd) {
		var run PipelineRun
		if err := json.Unmarshal(e.Data, &run); err != nil {
			return nil, err
		}
		if run.SpecID == specID {
			runs = append(runs, run)
		}
	}
	return runs, nil
}
