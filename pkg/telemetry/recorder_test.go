package telemetry_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecorder_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	r, err := telemetry.NewRecorder(dir, "SPEC-001")
	require.NoError(t, err)
	require.NotNil(t, r)

	telDir := filepath.Join(dir, ".autopus", "telemetry")
	info, err := os.Stat(telDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewRecorder_FileNameContainsSpecID(t *testing.T) {
	dir := t.TempDir()
	_, err := telemetry.NewRecorder(dir, "SPEC-TELE-001")
	require.NoError(t, err)

	telDir := filepath.Join(dir, ".autopus", "telemetry")
	entries, err := os.ReadDir(telDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Name(), "SPEC-TELE-001")
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".jsonl"))
}

func TestNewRecorder_PathTraversalPrevention(t *testing.T) {
	dir := t.TempDir()
	// specID with path traversal attempt
	r, err := telemetry.NewRecorder(dir, "../../../etc/passwd")
	require.NoError(t, err)
	require.NotNil(t, r)

	// File must be created inside the telemetry directory
	telDir := filepath.Join(dir, ".autopus", "telemetry")
	entries, err := os.ReadDir(telDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	// Must not contain path separators in filename
	assert.NotContains(t, entries[0].Name(), string(os.PathSeparator))
}

func TestRecorder_FullPipeline(t *testing.T) {
	dir := t.TempDir()
	r, err := telemetry.NewRecorder(dir, "SPEC-FULL-001")
	require.NoError(t, err)

	r.StartPipeline("SPEC-FULL-001", "strict")

	r.StartPhase("RED")
	r.RecordAgent(telemetry.AgentRun{
		AgentName: "tester",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(2 * time.Second),
		Duration:  2 * time.Second,
		Status:    telemetry.StatusPass,
	})
	r.EndPhase(telemetry.StatusPass)

	pipeline := r.Finalize(telemetry.StatusPass)

	assert.Equal(t, "SPEC-FULL-001", pipeline.SpecID)
	assert.Equal(t, telemetry.StatusPass, pipeline.FinalStatus)
	assert.Equal(t, "strict", pipeline.QualityMode)
	require.Len(t, pipeline.Phases, 1)
	assert.Equal(t, "RED", pipeline.Phases[0].Name)
	require.Len(t, pipeline.Phases[0].Agents, 1)
	assert.Equal(t, "tester", pipeline.Phases[0].Agents[0].AgentName)
}

func TestRecorder_WritesJSONLEvents(t *testing.T) {
	dir := t.TempDir()
	r, err := telemetry.NewRecorder(dir, "SPEC-JSONL-001")
	require.NoError(t, err)

	r.StartPipeline("SPEC-JSONL-001", "normal")
	r.StartPhase("GREEN")
	r.RecordAgent(telemetry.AgentRun{AgentName: "executor", Status: telemetry.StatusPass})
	r.EndPhase(telemetry.StatusPass)
	r.Finalize(telemetry.StatusPass)

	// Find the JSONL file
	telDir := filepath.Join(dir, ".autopus", "telemetry")
	entries, err := os.ReadDir(telDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	f, err := os.Open(filepath.Join(telDir, entries[0].Name()))
	require.NoError(t, err)
	defer f.Close()

	// Each line must be valid JSON with type and timestamp fields
	scanner := bufio.NewScanner(f)
	var eventTypes []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var event telemetry.Event
		require.NoError(t, json.Unmarshal([]byte(line), &event), "invalid JSON line: %s", line)
		assert.NotEmpty(t, event.Type)
		assert.False(t, event.Timestamp.IsZero())
		eventTypes = append(eventTypes, event.Type)
	}
	require.NoError(t, scanner.Err())

	assert.Contains(t, eventTypes, telemetry.EventTypePipelineStart)
	assert.Contains(t, eventTypes, telemetry.EventTypePhaseStart)
	assert.Contains(t, eventTypes, telemetry.EventTypeAgentRun)
	assert.Contains(t, eventTypes, telemetry.EventTypePhaseEnd)
	assert.Contains(t, eventTypes, telemetry.EventTypePipelineEnd)
}

func TestRecorder_MultiplePhases(t *testing.T) {
	dir := t.TempDir()
	r, err := telemetry.NewRecorder(dir, "SPEC-MULTI-001")
	require.NoError(t, err)

	r.StartPipeline("SPEC-MULTI-001", "strict")
	r.StartPhase("RED")
	r.EndPhase(telemetry.StatusPass)
	r.StartPhase("GREEN")
	r.EndPhase(telemetry.StatusPass)

	pipeline := r.Finalize(telemetry.StatusPass)
	require.Len(t, pipeline.Phases, 2)
	assert.Equal(t, "RED", pipeline.Phases[0].Name)
	assert.Equal(t, "GREEN", pipeline.Phases[1].Name)
}

func TestRecorder_CleanExpired(t *testing.T) {
	dir := t.TempDir()
	telDir := filepath.Join(dir, ".autopus", "telemetry")
	require.NoError(t, os.MkdirAll(telDir, 0o755))

	// Create an old file (simulate 10 days old)
	oldFile := filepath.Join(telDir, "2020-01-01-SPEC-OLD.jsonl")
	require.NoError(t, os.WriteFile(oldFile, []byte("{}"), 0o644))
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	// Create a recent file
	recentFile := filepath.Join(telDir, "2099-01-01-SPEC-NEW.jsonl")
	require.NoError(t, os.WriteFile(recentFile, []byte("{}"), 0o644))

	r, err := telemetry.NewRecorder(dir, "SPEC-CLEAN-001")
	require.NoError(t, err)

	err = r.CleanExpired(7)
	require.NoError(t, err)

	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "old file should be deleted")

	_, err = os.Stat(recentFile)
	assert.NoError(t, err, "recent file should still exist")
}

func TestRecorder_PipelineDurationIsPositive(t *testing.T) {
	dir := t.TempDir()
	r, err := telemetry.NewRecorder(dir, "SPEC-DUR-001")
	require.NoError(t, err)

	r.StartPipeline("SPEC-DUR-001", "normal")
	pipeline := r.Finalize(telemetry.StatusPass)

	assert.True(t, pipeline.TotalDuration >= 0)
	assert.False(t, pipeline.EndTime.IsZero())
}
