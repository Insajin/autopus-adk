package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func writeIndex(result Result, opts Options, started, ended time.Time) error {
	index := Index{
		SchemaVersion:       RunIndexSchemaVersion,
		RunID:               result.RunID,
		Status:              indexStatus(result),
		StartedAt:           started.Format(time.RFC3339Nano),
		EndedAt:             ended.Format(time.RFC3339Nano),
		Profile:             opts.Profile,
		Lane:                opts.Lane,
		ManifestPaths:       result.ManifestPaths,
		Checks:              result.Checks,
		AdapterResults:      result.AdapterResults,
		SetupGaps:           result.SetupGaps,
		FeedbackBundlePaths: result.FeedbackBundlePaths,
		RedactionStatus:     result.RedactionStatus,
	}
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(result.RunIndexPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(result.RunIndexPath, append(body, '\n'), 0o644)
}

func indexStatus(result Result) string {
	if result.Status == "warning" {
		return "warning"
	}
	return result.Status
}
