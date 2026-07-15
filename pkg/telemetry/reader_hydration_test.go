package telemetry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func TestLatestPipelineRun_EmptySummaryHydratesAgentEvents(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	start := time.Date(2026, 7, 12, 1, 0, 0, 0, time.UTC)
	agentStart := start.Add(time.Second)
	events := []telemetry.Event{
		hydrationEvent(t, telemetry.EventTypePipelineStart, start.Add(-time.Second), map[string]string{
			"spec_id": "OTHER", "quality_mode": "balanced",
		}),
		hydrationEvent(t, telemetry.EventTypePipelineStart, start, map[string]string{
			"spec_id": "SPEC-HYDRATE", "quality_mode": "ultra",
		}),
		hydrationEvent(t, telemetry.EventTypeAgentRun, agentStart, telemetry.AgentRun{
			AgentName: "ignored", SpecID: "OTHER", Phase: "review",
		}),
		{Type: telemetry.EventTypeAgentRun, Timestamp: agentStart, Data: json.RawMessage(`{"duration_ns":"invalid"}`)},
		hydrationEvent(t, telemetry.EventTypeAgentRun, agentStart, telemetry.AgentRun{
			AgentName: "executor", SpecID: "SPEC-HYDRATE", StartTime: agentStart,
			EndTime: agentStart.Add(2 * time.Second), Duration: 2 * time.Second, Status: telemetry.StatusPass,
		}),
		hydrationEvent(t, telemetry.EventTypeAgentRun, agentStart.Add(3*time.Second), telemetry.AgentRun{
			AgentName: "reviewer", SpecID: "SPEC-HYDRATE", Phase: "review", StartTime: agentStart.Add(3 * time.Second),
			EndTime: agentStart.Add(4 * time.Second), Duration: time.Second, Status: telemetry.StatusFail,
		}),
		hydrationEvent(t, telemetry.EventTypePipelineEnd, start.Add(10*time.Second), telemetry.PipelineRun{
			SpecID: "SPEC-HYDRATE", FinalStatus: telemetry.StatusPass,
		}),
	}
	writeHydrationEvents(t, baseDir, events)

	run, err := telemetry.LatestPipelineRun(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("LatestPipelineRun() returned nil")
	}
	if run.QualityMode != "ultra" || run.StartTime != start || run.TotalDuration != 10*time.Second {
		t.Fatalf("hydrated run metadata = %+v", *run)
	}
	if len(run.Phases) != 2 || run.Phases[0].Name != "agent" || run.Phases[1].Name != "review" {
		t.Fatalf("hydrated phases = %+v", run.Phases)
	}
	if len(run.Phases[0].Agents) != 1 || run.Phases[0].Agents[0].AgentName != "executor" {
		t.Fatalf("default phase agents = %+v", run.Phases[0].Agents)
	}
	if run.Phases[1].Status != telemetry.StatusFail || run.Phases[1].Duration != time.Second {
		t.Fatalf("review phase = %+v", run.Phases[1])
	}
}

func TestLatestPipelineRun_EmptySummaryWithoutMatchingStartRemainsUnhydrated(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	now := time.Now().UTC()
	writeHydrationEvents(t, baseDir, []telemetry.Event{
		hydrationEvent(t, telemetry.EventTypePipelineStart, now, map[string]string{"spec_id": "OTHER"}),
		hydrationEvent(t, telemetry.EventTypePipelineEnd, now.Add(time.Second), telemetry.PipelineRun{
			SpecID: "SPEC-NO-START", FinalStatus: telemetry.StatusPass,
		}),
	})

	run, err := telemetry.LatestPipelineRun(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || len(run.Phases) != 0 || !run.StartTime.IsZero() {
		t.Fatalf("unmatched start should not hydrate: %+v", run)
	}
}

func hydrationEvent(t *testing.T, eventType string, timestamp time.Time, value any) telemetry.Event {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return telemetry.Event{Type: eventType, Timestamp: timestamp, Data: data}
}

func writeHydrationEvents(t *testing.T, baseDir string, events []telemetry.Event) {
	t.Helper()
	dir := filepath.Join(baseDir, ".autopus", "telemetry")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, line...)
		data = append(data, '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "hydrate.jsonl"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
