package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

func TestHandleTask_EveryPipelinePhaseRetainsRequiredSnapshotAcrossChangingOutputs(t *testing.T) {
	fixture := writeRetainedWorkerContext(t)
	provider := &changingPhaseOutputAdapter{}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: fixture.root}}
	payload, err := json.Marshal(taskPayloadMessage{
		Prompt:         "RAW_PIPELINE_PROMPT",
		SpecID:         fixture.specID,
		PipelinePhases: []string{"planner", "executor", "tester", "reviewer"},
	})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "pipeline-required-context", payload)
	require.NoError(t, err)
	require.Len(t, provider.calls, 4)
	for _, call := range provider.calls {
		assertCompleteRequiredSpecSnapshot(t, call.Prompt, fixture.documents)
		assert.Equal(t, 1, strings.Count(call.Prompt, "RAW_PIPELINE_PROMPT"), "the original task must appear exactly once per phase")
		assert.Less(t, strings.Index(call.Prompt, "# Verified Required Context Snapshot"), strings.Index(call.Prompt, "RAW_PIPELINE_PROMPT"))
	}
	assert.Contains(t, provider.calls[0].Prompt, "RAW_PIPELINE_PROMPT")
	assert.Contains(t, provider.calls[1].Prompt, "PLANNER_OUTPUT_CHANGED")
	assert.Contains(t, provider.calls[2].Prompt, "EXECUTOR_OUTPUT_CHANGED")
	assert.Contains(t, provider.calls[3].Prompt, "TESTER_OUTPUT_CHANGED")
}

type changingPhaseOutputAdapter struct {
	calls []adapter.TaskConfig
}

func (a *changingPhaseOutputAdapter) Name() string { return "codex" }

func (a *changingPhaseOutputAdapter) BuildCommand(ctx context.Context, task adapter.TaskConfig) *exec.Cmd {
	a.calls = append(a.calls, task)
	output := strings.ToUpper(task.Phase) + "_OUTPUT_CHANGED"
	line, _ := json.Marshal(map[string]string{"type": "result", "output": output})
	script := fmt.Sprintf("cat >/dev/null; printf '%%s\\n' '%s'", line)
	return exec.CommandContext(ctx, "sh", "-c", script)
}

func (a *changingPhaseOutputAdapter) ParseEvent(line []byte) (adapter.StreamEvent, error) {
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		return adapter.StreamEvent{}, err
	}
	return adapter.StreamEvent{Type: event.Type, Data: append([]byte(nil), line...)}, nil
}

func (a *changingPhaseOutputAdapter) ExtractResult(event adapter.StreamEvent) adapter.TaskResult {
	var result struct {
		Output string `json:"output"`
	}
	_ = json.Unmarshal(event.Data, &result)
	return adapter.TaskResult{Output: result.Output}
}
