package pipeline_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestSubprocessEngine_NilBackendNonDryRun_FailsClosed(t *testing.T) {
	t.Parallel()

	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID:   "SPEC-ORCH-024",
		Platform: "codex",
		Strategy: pipeline.StrategySequential,
	})

	result, err := engine.Run(context.Background())

	require.EqualError(t, err, "pipeline: backend is required unless dry-run")
	assert.Nil(t, result)
}

func TestEvaluateGate_ConflictingOrUnknownVerdict_FailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gate   pipeline.GateType
		output string
	}{
		{
			name:   "validation conflict",
			gate:   pipeline.GateValidation,
			output: "VERDICT: PASS\nVERDICT: FAIL",
		},
		{
			name:   "review conflict",
			gate:   pipeline.GateReview,
			output: "VERDICT: APPROVE\nVERDICT: REQUEST_CHANGES",
		},
		{
			name:   "unknown gate",
			gate:   pipeline.GateType("future"),
			output: "VERDICT: PASS",
		},
		{
			name:   "pass substring",
			gate:   pipeline.GateValidation,
			output: "BYPASS",
		},
		{
			name:   "free form approval",
			gate:   pipeline.GateReview,
			output: "Looks good to me",
		},
		{
			name:   "negated approval",
			gate:   pipeline.GateReview,
			output: "NOT APPROVED",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, pipeline.VerdictFail, pipeline.EvaluateGate(tt.gate, tt.output))
		})
	}
}

func TestEvaluateGate_SingleExactTypedPass_Passes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, pipeline.VerdictPass,
		pipeline.EvaluateGate(pipeline.GateValidation, "VERDICT: PASS"))
	assert.Equal(t, pipeline.VerdictPass,
		pipeline.EvaluateGate(pipeline.GateReview, "VERDICT: APPROVE"))
}

type gateFailureBackend struct {
	mu    sync.Mutex
	calls []pipeline.PhaseID
}

func (b *gateFailureBackend) Execute(_ context.Context, req pipeline.PhaseRequest) (*pipeline.PhaseResponse, error) {
	b.mu.Lock()
	b.calls = append(b.calls, req.PhaseID)
	b.mu.Unlock()

	output := "phase complete"
	switch req.PhaseID {
	case pipeline.PhaseValidate:
		output = "VERDICT: FAIL"
	case pipeline.PhaseReview:
		output = "VERDICT: APPROVE"
	}
	return &pipeline.PhaseResponse{Output: output}, nil
}

func (b *gateFailureBackend) phases() []pipeline.PhaseID {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]pipeline.PhaseID(nil), b.calls...)
}

func TestSubprocessEngine_ValidateGateFails_DoesNotDispatchReview(t *testing.T) {
	t.Parallel()

	backend := &gateFailureBackend{}
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID:   "SPEC-ORCH-024",
		Platform: "codex",
		Strategy: pipeline.StrategySequential,
		Backend:  backend,
	})

	_, err := engine.Run(context.Background())

	assert.Error(t, err)
	phases := backend.phases()
	assert.Contains(t, phases, pipeline.PhaseValidate)
	assert.NotContains(t, phases, pipeline.PhaseReview)
}

func TestOrchestrationRunReceipt_UsesCanonicalSchemaAndTerminalKeys(t *testing.T) {
	t.Parallel()

	receipt := pipeline.NewBlockedRunReceipt("SPEC-MISSING", pipeline.StrategySequential, "missing SPEC")
	data, err := json.Marshal(receipt)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))

	assert.Equal(t, pipeline.OrchestrationRunReceiptVersion, payload["schema"])
	assert.Equal(t, string(pipeline.TerminalBlocked), payload["terminal_state"])
	assert.NotContains(t, payload, "terminal")
	assert.NotContains(t, payload, "schema_version")
}

func TestSubprocessEngine_DryRunCheckpointAndDashboardPreserveSkippedStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID: "SPEC-DRY-001", Platform: "codex",
		Strategy: pipeline.StrategySequential, DryRun: true,
		RunConfig: pipeline.RunConfig{SpecID: "SPEC-DRY-001", CheckpointDir: dir},
	})
	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, pipeline.TerminalDryRun, result.Receipt.Terminal)
	assert.Zero(t, result.Receipt.DispatchCount)

	cp, err := pipeline.LoadFile(filepath.Join(dir, "SPEC-DRY-001.yaml"))
	require.NoError(t, err)
	dashboard := pipeline.MapCheckpointToPhases(cp)
	for _, phase := range pipeline.DefaultPhases() {
		assert.Equal(t, pipeline.CheckpointStatusSkipped, cp.TaskStatus[string(phase.ID)])
		assert.Equal(t, pipeline.PhaseSkipped, dashboard.Phases[string(phase.ID)])
	}
}
