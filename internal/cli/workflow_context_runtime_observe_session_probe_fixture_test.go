package cli

// Fixtures for the REQ-PROBE-001 cohort tests. They live in a sibling file so
// the test file that drives them stays inside the source-size limit.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/require"
)

type workflowContextObserveProbeRun struct {
	records    []ompprobe.Record
	responses  []workflowContextObserveSessionResponse
	body       []byte
	options    workflowContextObserveSessionOptions
	probePath  string
	taskBodies []string
	err        error
}

// runWorkflowContextObserveProbe drives one probe cohort against the fake OMP.
// callLimit > 0 cuts the input short so the probe aborts mid-cohort.
func runWorkflowContextObserveProbe(t *testing.T, model string, callLimit int) workflowContextObserveProbeRun {
	t.Helper()
	requireDarwinManagedOMPSandboxForTest(t)
	challenge := workflowContextRuntimeHash("observe-session-probe-" + model)
	setup, options, _ := workflowContextObserveSessionFixtureWithModel(t, challenge, model)
	options.ProbeDir = filepath.Join(t.TempDir(), "probe")
	input, taskBodies := workflowContextObserveEvidenceInput(t, challenge)
	if callLimit > 0 {
		input = truncateWorkflowContextObserveInput(t, input, callLimit)
	}
	var output bytes.Buffer
	prepared := &setup
	ctx := context.WithValue(context.Background(), workflowContextObserveSessionPrepareKey{},
		workflowContextObserveSessionPrepare(func(
			_ context.Context, _ workflowContextObserveSessionOptions, _ string,
		) (workflowContextObserveSessionSetup, error) {
			return *prepared, nil
		}))
	runErr := RunWorkflowContextObserveSession(ctx, input, &output, options)
	probePath := filepath.Join(options.ProbeDir, "probe.jsonl")
	body, readErr := os.ReadFile(probePath)
	require.NoError(t, readErr, "an interrupted probe still retains its records")
	return workflowContextObserveProbeRun{
		records:   decodeWorkflowContextObserveProbeRecords(t, body),
		responses: decodeWorkflowContextObserveResponses(t, output.Bytes()),
		body:      body, options: options, probePath: probePath,
		taskBodies: taskBodies, err: runErr,
	}
}

// truncateWorkflowContextObserveInput keeps the handshake and the first calls,
// so the run dies with the input exhausted instead of shutting down.
func truncateWorkflowContextObserveInput(t *testing.T, input *bytes.Buffer, calls int) *bytes.Buffer {
	t.Helper()
	lines := strings.SplitAfter(input.String(), "\n")
	require.Greater(t, len(lines), calls+1)
	return bytes.NewBufferString(strings.Join(lines[:calls+1], ""))
}

func decodeWorkflowContextObserveProbeRecords(t *testing.T, body []byte) []ompprobe.Record {
	t.Helper()
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	records := make([]ompprobe.Record, 0, len(lines))
	for _, line := range lines {
		var record ompprobe.Record
		require.NoError(t, json.Unmarshal([]byte(line), &record), "probe record %q", line)
		records = append(records, record)
	}
	return records
}

func workflowContextObserveProbeRecordsOfKind(records []ompprobe.Record, kind string) []ompprobe.Record {
	selected := make([]ompprobe.Record, 0, len(records))
	for _, record := range records {
		if record.Kind == kind {
			selected = append(selected, record)
		}
	}
	return selected
}
