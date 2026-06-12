package worker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- SetEnvVars ----------

func TestSetEnvVars_NilAndEmpty(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	pe.SetEnvVars(nil)
	assert.Nil(t, pe.envVars)

	pe.SetEnvVars(map[string]string{})
	assert.Nil(t, pe.envVars)
}

func TestSetEnvVars_Populated(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	pe.SetEnvVars(map[string]string{"KEY": "value", "FOO": "bar"})

	require.NotNil(t, pe.envVars)
	assert.Equal(t, "value", pe.envVars["KEY"])
	assert.Equal(t, "bar", pe.envVars["FOO"])
}

// ---------- completePhase ----------

func TestCompletePhase_WithAndWithoutAllocator(t *testing.T) {
	t.Parallel()

	// Without allocator: no panic, no-op.
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	pe.completePhase(PhasePlanner, 5) // must not panic

	// With allocator: tool-call counts are tracked.
	pe.SetBudget(100, budget.DefaultAllocation())
	pe.completePhase(PhasePlanner, 10)
	remaining := pe.allocator.TotalRemaining()
	assert.Equal(t, 90, remaining)
}

// ---------- logPhaseBudget ----------

func TestLogPhaseBudget_NilAllocatorNoOp(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	// logPhaseBudget with nil allocator must not panic.
	pe.logPhaseBudget(PhaseExecutor)
}

// ---------- resolveModel ----------

func TestResolveModel_ExplicitModelWins(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	// No router: explicit model is returned as-is.
	assert.Equal(t, "gpt-4o", pe.resolveModel("gpt-4o", "some prompt"))
}

func TestResolveModel_EmptyModelNoRouterReturnsEmpty(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	// No router configured — returns empty string for empty model.
	assert.Equal(t, "", pe.resolveModel("", "some prompt"))
}

// ---------- renderPhasePromptTemplate ----------

func TestRenderPhasePromptTemplate_WithPlaceholder(t *testing.T) {
	t.Parallel()

	result := renderPhasePromptTemplate("PREFIX\n\n{{input}}", "my input")
	assert.Equal(t, "PREFIX\n\nmy input", result)
}

func TestRenderPhasePromptTemplate_WithoutPlaceholderAppends(t *testing.T) {
	t.Parallel()

	result := renderPhasePromptTemplate("PREFIX", "my input")
	assert.Equal(t, "PREFIX\n\nmy input", result)
}

// ---------- parsePhaseStream ----------

func TestParsePhaseStream_SingleResultEvent(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(&mockAdapter{
		name: "mock",
		parseFn: func(line []byte) (adapter.StreamEvent, error) {
			var v struct{ Type string }
			_ = json.Unmarshal(line, &v)
			return adapter.StreamEvent{Type: v.Type, Data: line}, nil
		},
		extractFn: func(evt adapter.StreamEvent) adapter.TaskResult {
			var d struct {
				Output string `json:"output"`
			}
			_ = json.Unmarshal(evt.Data, &d)
			return adapter.TaskResult{Output: d.Output, CostUSD: 0.01, DurationMS: 100}
		},
	}, "", t.TempDir())

	stream := strings.NewReader(`{"type":"result","output":"done"}` + "\n")
	result, err := pe.parsePhaseStream(stream, "t1", PhasePlanner, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "done", result.Output)
	assert.Equal(t, 0.01, result.CostUSD)
	assert.Equal(t, int64(100), result.DurationMS)
}

func TestParsePhaseStream_NoResultEventErrors(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(&mockAdapter{
		name: "mock",
		parseFn: func(line []byte) (adapter.StreamEvent, error) {
			return adapter.StreamEvent{Type: "tool_call", Data: line}, nil
		},
	}, "", t.TempDir())

	stream := strings.NewReader(`{"type":"tool_call"}` + "\n")
	_, err := pe.parsePhaseStream(stream, "t1", PhasePlanner, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result event")
}

func TestParsePhaseStream_CountsToolCalls(t *testing.T) {
	t.Parallel()

	calls := 0
	pe := NewPipelineExecutor(&mockAdapter{
		name: "mock",
		parseFn: func(line []byte) (adapter.StreamEvent, error) {
			var v struct{ Type string }
			_ = json.Unmarshal(line, &v)
			return adapter.StreamEvent{Type: v.Type, Data: line}, nil
		},
		extractFn: func(evt adapter.StreamEvent) adapter.TaskResult {
			calls++
			return adapter.TaskResult{Output: "result"}
		},
	}, "", t.TempDir())

	streamText := `{"type":"tool_call"}` + "\n" +
		`{"type":"tool_use"}` + "\n" +
		`{"type":"result","output":"result"}` + "\n"

	result, err := pe.parsePhaseStream(strings.NewReader(streamText), "t1", PhaseExecutor, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.ToolCalls)
}

// ---------- phaseIterationBudget ----------

func TestPhaseIterationBudget_NilWhenNoConfig(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	assert.Nil(t, pe.phaseIterationBudget(PhasePlanner))
}

func TestPhaseIterationBudget_ReturnsScaledLimit(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	pe.SetIterationBudget(budget.IterationBudget{
		Limit:           100,
		WarnThreshold:   0.7,
		DangerThreshold: 0.9,
	})

	b := pe.phaseIterationBudget(PhasePlanner)
	require.NotNil(t, b)
	assert.Greater(t, b.Limit, 0)
}

// ---------- aggregateResults ----------

func TestAggregateResults_CombinesPhases(t *testing.T) {
	t.Parallel()

	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())
	results := []PhaseResult{
		{Phase: PhasePlanner, Output: "plan output", CostUSD: 0.01, DurationMS: 100},
		{Phase: PhaseExecutor, Output: "exec output", CostUSD: 0.02, DurationMS: 200},
	}
	tr := pe.aggregateResults(results, 0.03, 300)

	assert.InDelta(t, 0.03, tr.CostUSD, 0.0001)
	assert.Equal(t, int64(300), tr.DurationMS)
	assert.Contains(t, tr.Output, "## Phase: planner")
	assert.Contains(t, tr.Output, "plan output")
	assert.Contains(t, tr.Output, "## Phase: executor")
	assert.Contains(t, tr.Output, "exec output")
}

// ---------- normalizePhasePlan ----------

func TestNormalizePhasePlan_EmptyUsesDefault(t *testing.T) {
	t.Parallel()

	plan := normalizePhasePlan(nil)
	assert.Equal(t, defaultPipelinePhases, plan)
}

func TestNormalizePhasePlan_PreservesExplicit(t *testing.T) {
	t.Parallel()

	explicit := []Phase{PhasePlanner, PhaseReviewer}
	plan := normalizePhasePlan(explicit)
	assert.Equal(t, explicit, plan)
	// Ensure it is a copy, not the same slice.
	plan[0] = PhaseExecutor
	assert.Equal(t, PhasePlanner, explicit[0])
}
