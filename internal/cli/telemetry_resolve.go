package cli

import (
	"encoding/json"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// resolveSingleRun returns the latest PipelineRun, optionally filtered by specID.
func resolveSingleRun(baseDir, specID string) (*telemetry.PipelineRun, error) {
	if specID != "" {
		runs, err := telemetry.PipelineRunsBySpecID(baseDir, specID)
		if err != nil {
			return nil, fmt.Errorf("telemetry: load runs: %w", err)
		}
		if len(runs) == 0 {
			return nil, fmt.Errorf("telemetry: no runs found for spec-id %q", specID)
		}
		run := runs[len(runs)-1]
		return &run, nil
	}

	run, err := telemetry.LatestPipelineRun(baseDir)
	if err != nil {
		return nil, fmt.Errorf("telemetry: load latest run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("telemetry: no pipeline runs found")
	}
	return run, nil
}

// resolveTwoRuns returns the two most recent PipelineRuns for comparison,
// optionally filtered by specID.
func resolveTwoRuns(baseDir, specID string) ([]telemetry.PipelineRun, error) {
	var runs []telemetry.PipelineRun

	if specID != "" {
		r, err := telemetry.PipelineRunsBySpecID(baseDir, specID)
		if err != nil {
			return nil, fmt.Errorf("telemetry: load runs: %w", err)
		}
		runs = r
	} else {
		// Load all events and collect pipeline_end records.
		events, err := telemetry.LoadAllEvents(baseDir)
		if err != nil {
			return nil, fmt.Errorf("telemetry: load events: %w", err)
		}
		for _, e := range telemetry.FilterByType(events, "pipeline_end") {
			var run telemetry.PipelineRun
			if err := json.Unmarshal(e.Data, &run); err != nil {
				return nil, fmt.Errorf("telemetry: decode run: %w", err)
			}
			runs = append(runs, run)
		}
	}

	if len(runs) < 2 {
		return nil, fmt.Errorf("telemetry: need at least 2 runs to compare, found %d", len(runs))
	}
	// Return the two most recent.
	return runs[len(runs)-2:], nil
}
