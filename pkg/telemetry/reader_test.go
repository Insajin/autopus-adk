package telemetry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTelemetryJSONL writes events as JSONL to a file under baseDir/.autopus/telemetry/.
func writeTelemetryJSONL(t *testing.T, baseDir, filename string, events []telemetry.Event) string {
	t.Helper()
	dir := filepath.Join(baseDir, ".autopus", "telemetry")
	require.NoError(t, os.MkdirAll(dir, 0755))
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	for _, e := range events {
		line, err := json.Marshal(e)
		require.NoError(t, err)
		_, err = f.Write(append(line, '\n'))
		require.NoError(t, err)
	}
	return path
}

// makePipelineEvent builds a pipeline_end Event for a given specID and timestamp.
func makePipelineEvent(t *testing.T, specID string, ts time.Time) telemetry.Event {
	t.Helper()
	run := telemetry.PipelineRun{
		SpecID:      specID,
		StartTime:   ts,
		EndTime:     ts.Add(10 * time.Second),
		FinalStatus: telemetry.StatusPass,
		QualityMode: "strict",
	}
	data, err := json.Marshal(run)
	require.NoError(t, err)
	return telemetry.Event{
		Type:      telemetry.EventTypePipelineEnd,
		Timestamp: ts,
		Data:      data,
	}
}

func TestLoadEvents_ReturnsEvents(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Millisecond)
	events := []telemetry.Event{
		{Type: telemetry.EventTypeAgentRun, Timestamp: now, Data: json.RawMessage(`{}`)},
		{Type: telemetry.EventTypePipelineEnd, Timestamp: now.Add(time.Second), Data: json.RawMessage(`{}`)},
	}
	path := writeTelemetryJSONL(t, dir, "run.jsonl", events)

	got, err := telemetry.LoadEvents(path)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, telemetry.EventTypeAgentRun, got[0].Type)
	assert.Equal(t, telemetry.EventTypePipelineEnd, got[1].Type)
}

func TestLoadEvents_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	content := `{"type":"agent_run","timestamp":"2024-01-01T00:00:00Z","data":{}}` + "\n\n" +
		`{"type":"pipeline_end","timestamp":"2024-01-01T00:00:01Z","data":{}}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	got, err := telemetry.LoadEvents(path)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestLoadEvents_FileNotFound_ReturnsEmpty(t *testing.T) {
	got, err := telemetry.LoadEvents("/nonexistent/path/events.jsonl")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestLoadAllEvents_MergesAndSortsByTimestamp(t *testing.T) {
	baseDir := t.TempDir()
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 0, 0, 2, 0, time.UTC)

	writeTelemetryJSONL(t, baseDir, "a.jsonl", []telemetry.Event{
		{Type: telemetry.EventTypeAgentRun, Timestamp: t3, Data: json.RawMessage(`{}`)},
		{Type: telemetry.EventTypeAgentRun, Timestamp: t1, Data: json.RawMessage(`{}`)},
	})
	writeTelemetryJSONL(t, baseDir, "b.jsonl", []telemetry.Event{
		{Type: telemetry.EventTypeAgentRun, Timestamp: t2, Data: json.RawMessage(`{}`)},
	})

	got, err := telemetry.LoadAllEvents(baseDir)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.True(t, got[0].Timestamp.Before(got[1].Timestamp))
	assert.True(t, got[1].Timestamp.Before(got[2].Timestamp))
}

func TestLoadAllEvents_NoDirReturnsEmpty(t *testing.T) {
	baseDir := t.TempDir()
	// No .autopus/telemetry/ dir created.
	got, err := telemetry.LoadAllEvents(baseDir)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFilterByType(t *testing.T) {
	events := []telemetry.Event{
		{Type: telemetry.EventTypeAgentRun, Timestamp: time.Now()},
		{Type: telemetry.EventTypePipelineEnd, Timestamp: time.Now()},
		{Type: telemetry.EventTypeAgentRun, Timestamp: time.Now()},
	}

	got := telemetry.FilterByType(events, telemetry.EventTypeAgentRun)
	assert.Len(t, got, 2)

	got = telemetry.FilterByType(events, telemetry.EventTypePipelineEnd)
	assert.Len(t, got, 1)

	got = telemetry.FilterByType(events, "unknown")
	assert.Empty(t, got)
}

func TestLatestPipelineRun_ReturnsMostRecent(t *testing.T) {
	baseDir := t.TempDir()
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	writeTelemetryJSONL(t, baseDir, "runs.jsonl", []telemetry.Event{
		makePipelineEvent(t, "SPEC-001", t1),
		makePipelineEvent(t, "SPEC-002", t2),
	})

	run, err := telemetry.LatestPipelineRun(baseDir)
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "SPEC-002", run.SpecID)
}

func TestLatestPipelineRun_NoPipelineEvents_ReturnsNil(t *testing.T) {
	baseDir := t.TempDir()
	writeTelemetryJSONL(t, baseDir, "runs.jsonl", []telemetry.Event{
		{Type: telemetry.EventTypeAgentRun, Timestamp: time.Now(), Data: json.RawMessage(`{}`)},
	})

	run, err := telemetry.LatestPipelineRun(baseDir)
	require.NoError(t, err)
	assert.Nil(t, run)
}

func TestLatestPipelineRun_EmptyDir_ReturnsNil(t *testing.T) {
	baseDir := t.TempDir()
	run, err := telemetry.LatestPipelineRun(baseDir)
	require.NoError(t, err)
	assert.Nil(t, run)
}

func TestPipelineRunsBySpecID_FiltersCorrectly(t *testing.T) {
	baseDir := t.TempDir()
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	writeTelemetryJSONL(t, baseDir, "runs.jsonl", []telemetry.Event{
		makePipelineEvent(t, "SPEC-001", t1),
		makePipelineEvent(t, "SPEC-002", t2),
		makePipelineEvent(t, "SPEC-001", t3),
	})

	runs, err := telemetry.PipelineRunsBySpecID(baseDir, "SPEC-001")
	require.NoError(t, err)
	assert.Len(t, runs, 2)
	for _, r := range runs {
		assert.Equal(t, "SPEC-001", r.SpecID)
	}
}

func TestPipelineRunsBySpecID_NoMatch_ReturnsEmpty(t *testing.T) {
	baseDir := t.TempDir()
	writeTelemetryJSONL(t, baseDir, "runs.jsonl", []telemetry.Event{
		makePipelineEvent(t, "SPEC-001", time.Now()),
	})

	runs, err := telemetry.PipelineRunsBySpecID(baseDir, "SPEC-999")
	require.NoError(t, err)
	assert.Empty(t, runs)
}
