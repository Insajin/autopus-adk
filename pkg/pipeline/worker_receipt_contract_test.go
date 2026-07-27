package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/workerreceipt"
)

var _ workerreceipt.Receipt = pipeline.WorkerRunReceipt{}
var _ pipeline.WorkerRunReceipt = workerreceipt.Receipt{}

func TestPipeline_ValidExplicitWorkerReceiptIsAppendedExactlyOnce(t *testing.T) {
	t.Parallel()

	want := pipelineWorkerReceiptFixture()
	backend := &workerReceiptBackend{outputs: []string{
		pipelineMarkedReceipt(t, want),
		"test scaffold complete",
		"implementation complete",
		"VERDICT: PASS",
		"VERDICT: APPROVE",
	}}
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID:   "SPEC-WORKER-RECEIPT-001",
		Platform: "plain",
		Strategy: pipeline.StrategySequential,
		Backend:  backend,
	})

	result, err := engine.Run(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Receipt.WorkerReceipts, 1)
	assert.Equal(t, want, result.Receipt.WorkerReceipts[0])

	persisted, err := json.Marshal(result.Receipt)
	require.NoError(t, err)
	var raw struct {
		WorkerReceipts []workerreceipt.Receipt `json:"worker_receipts"`
	}
	require.NoError(t, json.Unmarshal(persisted, &raw))
	assert.Equal(t, []workerreceipt.Receipt{want}, raw.WorkerReceipts)
}

func TestPipeline_MarkerlessReceiptLikeDebateOutputPreservesEmptyReceipts(t *testing.T) {
	t.Parallel()

	debateJSON, err := json.Marshal(pipelineWorkerReceiptFixture())
	require.NoError(t, err)
	backend := &workerReceiptBackend{outputs: []string{
		"orchestra debate evidence only\n" + string(debateJSON),
		"test scaffold complete",
		"implementation complete",
		"VERDICT: PASS",
		"VERDICT: APPROVE",
	}}
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID:   "SPEC-WORKER-RECEIPT-002",
		Platform: "plain",
		Strategy: pipeline.StrategySequential,
		Backend:  backend,
	})

	result, err := engine.Run(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Receipt.WorkerReceipts)
	assert.NotNil(t, result.Receipt.WorkerReceipts)
}

func TestPipeline_DuplicateWorkerReceiptMarkersFailEvidenceGateWithoutAppend(t *testing.T) {
	t.Parallel()

	marked := pipelineMarkedReceipt(t, pipelineWorkerReceiptFixture())
	backend := &workerReceiptBackend{outputs: []string{marked + "\n" + marked}}
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID:   "SPEC-WORKER-RECEIPT-003",
		Platform: "plain",
		Strategy: pipeline.StrategySequential,
		Backend:  backend,
	})

	result, err := engine.Run(context.Background())

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Receipt.WorkerReceipts)
	assert.Equal(t, pipeline.TerminalBlocked, result.Receipt.Terminal)
	assert.Contains(t, result.Receipt.Blocker, "worker receipt")
}

func TestPipeline_UnsafeWorkerReceiptTextNeverReachesPersistedCheckpoint(t *testing.T) {
	t.Parallel()

	checkpointDir := t.TempDir()
	secret := "sk-proj-checkpoint-secret-abcdefghijklmnopqrstuvwxyz"
	receipt := pipelineWorkerReceiptFixture()
	receipt.Verification = []string{"OPENAI_API_KEY=" + secret}
	backend := &workerReceiptBackend{outputs: []string{pipelineMarkedReceipt(t, receipt)}}
	engine := pipeline.NewSubprocessEngine(pipeline.EngineConfig{
		SpecID:   "SPEC-WORKER-RECEIPT-PRIVACY-001",
		Platform: "plain",
		Strategy: pipeline.StrategySequential,
		Backend:  backend,
		RunConfig: pipeline.RunConfig{
			SpecID:        "SPEC-WORKER-RECEIPT-PRIVACY-001",
			CheckpointDir: checkpointDir,
		},
	})

	result, err := engine.Run(context.Background())

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Receipt.WorkerReceipts)
	persisted, readErr := os.ReadFile(filepath.Join(
		checkpointDir, "SPEC-WORKER-RECEIPT-PRIVACY-001.yaml",
	))
	require.NoError(t, readErr)
	assert.NotContains(t, string(persisted), secret)
	assert.NotContains(t, string(persisted), "OPENAI_API_KEY")
}

type workerReceiptBackend struct {
	mu      sync.Mutex
	outputs []string
	next    int
}

func (b *workerReceiptBackend) Execute(_ context.Context, _ pipeline.PhaseRequest) (*pipeline.PhaseResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.next >= len(b.outputs) {
		return &pipeline.PhaseResponse{Output: "unexpected extra phase"}, nil
	}
	output := b.outputs[b.next]
	b.next++
	return &pipeline.PhaseResponse{Output: output}, nil
}

func pipelineWorkerReceiptFixture() workerreceipt.Receipt {
	return workerreceipt.Receipt{
		OwnedPaths:       []string{"pkg/pipeline"},
		ChangedFiles:     []string{"pkg/pipeline/engine_run.go"},
		Verification:     []string{"go test ./pkg/pipeline"},
		Blockers:         []string{},
		NextRequiredStep: "review",
	}
}

func pipelineMarkedReceipt(t *testing.T, receipt workerreceipt.Receipt) string {
	t.Helper()
	envelope := workerreceipt.Envelope{
		SchemaVersion: workerreceipt.SchemaVersion,
		Receipt:       receipt,
	}
	data, err := json.Marshal(envelope)
	require.NoError(t, err)
	return fmt.Sprintf("%s\n%s\n%s", workerreceipt.BeginMarker, data, workerreceipt.EndMarker)
}
