package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppendAgentRun persists one final agent event without retaining a file
// handle. It is the lightweight worker-host persistence boundary.
func AppendAgentRun(baseDir string, run AgentRun) (err error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	specID := filepath.Base(filepath.Clean(strings.TrimSpace(run.SpecID)))
	if specID == "" || specID == "." || specID == string(filepath.Separator) {
		specID = "unknown"
	}
	dir := filepath.Join(baseDir, telemetrySubDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("telemetry: create append dir: %w", err)
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("telemetry: marshal agent run: %w", err)
	}
	event, err := json.Marshal(Event{Type: EventTypeAgentRun, Timestamp: time.Now().UTC(), Data: payload})
	if err != nil {
		return fmt.Errorf("telemetry: marshal agent event: %w", err)
	}
	path := filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+"-"+specID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("telemetry: open append file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("telemetry: close append file: %w", closeErr)
		}
	}()
	if _, err = file.Write(append(event, '\n')); err != nil {
		return fmt.Errorf("telemetry: append agent event: %w", err)
	}
	return nil
}
