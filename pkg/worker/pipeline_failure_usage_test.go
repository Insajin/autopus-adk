package worker

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/compress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineExecuteWithPlan_AfterActualUsageError_PreservesSpend(t *testing.T) {
	tests := []struct {
		name    string
		phases  []Phase
		failure string
		wantErr string
	}{
		{name: "compaction blocker", phases: []Phase{PhasePlanner, PhaseExecutor}, failure: "block", wantErr: "context-budget"},
		{name: "phase prompt", phases: []Phase{PhasePlanner, Phase("unsupported")}, wantErr: "unsupported phase"},
		{name: "context cancellation", phases: []Phase{PhasePlanner, PhaseExecutor}, failure: "cancel", wantErr: "context canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := pipelineActualUsageAdapter("phase output", false)
			pe := NewPipelineExecutor(mock, "", t.TempDir())
			ctx := context.Background()
			if tt.failure == "block" {
				pe.SetCompressor(pipelineFailureCompressor{blocker: "context-budget"})
			}
			if tt.failure == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				pe.SetCompressor(pipelineFailureCompressor{cancel: cancel})
			}

			result, err := pe.ExecuteWithPlan(ctx, "failure-task", "prompt", "model", tt.phases)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assertPipelineActualSpend(t, result)
		})
	}
}

func TestPipelineExecuteWithPlan_EmptyOutputFailedSubprocess_PreservesActualUsage(t *testing.T) {
	mock := pipelineActualUsageAdapter("", true)
	pe := NewPipelineExecutor(mock, "", t.TempDir())

	result, err := pe.ExecuteWithPlan(context.Background(), "failed-task", "prompt", "model", []Phase{PhasePlanner})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "subprocess exit")
	assertPipelineActualSpend(t, result)
}

type pipelineFailureCompressor struct {
	blocker string
	cancel  context.CancelFunc
}

func (c pipelineFailureCompressor) Compress(_, output, _ string) string { return output }

func (c pipelineFailureCompressor) CompressDetailed(_, output, _ string) compress.CompactionResult {
	if c.cancel != nil {
		c.cancel()
	}
	return compress.CompactionResult{Output: output, Blocker: c.blocker}
}

func pipelineActualUsageAdapter(output string, fail bool) *pipelineMockAdapter {
	usage := workerUsage("provider-run", "provider-call", 40, 10)
	script := `cat /dev/stdin >/dev/null; echo result`
	if fail {
		script += `; exit 1`
	}
	return &pipelineMockAdapter{
		script: script,
		parseFn: func([]byte) (adapter.StreamEvent, error) {
			return adapter.StreamEvent{Type: "result", Usage: []telemetry.UsageEnvelope{usage}}, nil
		},
		extractFn: func(event adapter.StreamEvent) adapter.TaskResult {
			return adapter.TaskResult{Output: output, CostUSD: 0.25, DurationMS: 75, Usage: event.Usage}
		},
	}
}

func assertPipelineActualSpend(t *testing.T, result adapter.TaskResult) {
	t.Helper()
	assert.InDelta(t, 0.25, result.CostUSD, 0.0001)
	assert.Equal(t, int64(75), result.DurationMS)
	require.Len(t, result.Usage, 1)
	assert.Equal(t, telemetry.UsageStatusActual, result.Usage[0].UsageStatus)
	assert.NotNil(t, result.Usage[0].RawTotalTokens)
	assert.Equal(t, int64(50), *result.Usage[0].RawTotalTokens)
	assert.Contains(t, result.Output, "planner")
}
