package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type receiptEvidenceBackend struct {
	mu       sync.Mutex
	calls    []ProviderRequest
	failRole string
	failName string
}

func (b *receiptEvidenceBackend) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	b.mu.Lock()
	b.calls = append(b.calls, req)
	b.mu.Unlock()
	resp := &ProviderResponse{
		Provider: req.Provider, Output: receiptContractOutput(req.Role),
		ExecutedBackend: "recording", Receipt: fmt.Sprintf("artifact/%s/%d", req.Role, req.Round),
		Role: req.Role, Attempt: req.Round, ModelFamily: req.Config.ModelFamily,
	}
	if (b.failRole == "" || b.failRole == req.Role) && (b.failName == "*" || b.failName == req.Provider) {
		resp.ExitCode = 23
		resp.Error = "recorded failure"
		return resp, fmt.Errorf("%s/%s failed", req.Role, req.Provider)
	}
	return resp, nil
}

func (b *receiptEvidenceBackend) Name() string { return "recording" }

func (b *receiptEvidenceBackend) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.calls)
}

func receiptContractOutput(role string) string {
	switch role {
	case "judge":
		data, _ := json.Marshal(JudgeOutput{Recommendation: "proceed"})
		return string(data)
	case "debater_r2":
		data, _ := json.Marshal(DebaterR2Output{})
		return string(data)
	default:
		data, _ := json.Marshal(DebaterR1Output{Ideas: []IdeaOutput{{Title: "idea"}}})
		return string(data)
	}
}

func receiptContractPipeline(backend ExecutionBackend) SubprocessPipelineConfig {
	return SubprocessPipelineConfig{
		Backend: backend,
		Providers: []ProviderConfig{
			{Name: "claude", ModelFamily: "anthropic"},
			{Name: "codex", ModelFamily: "openai"},
			{Name: "gemini", ModelFamily: "google"},
		},
		Topic: "receipt contract",
		PromptData: PromptData{
			ProjectName: "autopus", ProjectSummary: "contract", TechStack: "Go",
			MustReadFiles: []string{"go.mod"}, Topic: "receipt contract",
		},
		Rounds:                       1,
		Judge:                        ProviderConfig{Name: "judge", ModelFamily: "independent"},
		RequireJudgeFamilySeparation: true,
	}
}

func TestRunSubprocessPipeline_DispatchReceipts_PreserveExecutionEvidence(t *testing.T) {
	backend := &receiptEvidenceBackend{failRole: "debater_r2", failName: "gemini"}

	result, err := RunSubprocessPipeline(context.Background(), receiptContractPipeline(backend))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.RunReceipt)
	assert.Equal(t, 7, backend.callCount())
	assert.Equal(t, 7, result.DispatchCount)
	assert.Len(t, result.RunReceipt.ProviderReceipts, result.DispatchCount)

	roleCounts := map[string]int{}
	for _, receipt := range result.RunReceipt.ProviderReceipts {
		roleCounts[receipt.Role]++
		assert.Equal(t, "recording", receipt.Backend)
		assert.NotEmpty(t, receipt.ModelFamily)
		assert.NotZero(t, receipt.Attempt)
		assert.NotEmpty(t, receipt.Artifact)
	}
	assert.Equal(t, 3, roleCounts["debater_r1"])
	assert.Equal(t, 3, roleCounts["debater_r2"])
	assert.Equal(t, 1, roleCounts["judge"])

	var failedR2 *ProviderRunReceipt
	for i := range result.RunReceipt.ProviderReceipts {
		receipt := &result.RunReceipt.ProviderReceipts[i]
		if receipt.Provider == "gemini" && receipt.Role == "debater_r2" {
			failedR2 = receipt
			break
		}
	}
	require.NotNil(t, failedR2)
	assert.Equal(t, 2, failedR2.Attempt)
	assert.Equal(t, 23, failedR2.ExitCode)
	assert.False(t, failedR2.Usable)
}

func TestRunSubprocessPipeline_AllParticipantsFail_ReturnsBlockedReceipt(t *testing.T) {
	backend := &receiptEvidenceBackend{failRole: "debater_r1", failName: "*"}
	cfg := receiptContractPipeline(backend)
	cfg.Rounds = 0

	result, err := RunSubprocessPipeline(context.Background(), cfg)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, TerminalBlocked, result.TerminalState)
	assert.Equal(t, "blocked", result.GateStatus)
	assert.Equal(t, 3, result.DispatchCount)
	require.NotNil(t, result.RunReceipt)
	assert.Len(t, result.RunReceipt.ProviderReceipts, 3)
	assert.Equal(t, 3, backend.callCount())
}

func TestRunSubprocessPipeline_ProviderRecovers_IsUsableForQuorum(t *testing.T) {
	backend := &receiptEvidenceBackend{failRole: "debater_r1", failName: "gemini"}

	result, err := RunSubprocessPipeline(context.Background(), receiptContractPipeline(backend))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.FailedProviderNames, "gemini")
	assert.Contains(t, result.UsableProviders, "gemini")
	assert.True(t, result.QuorumMet)
	geminiAttempts := 0
	for _, receipt := range result.RunReceipt.ProviderReceipts {
		if receipt.Provider == "gemini" {
			geminiAttempts++
		}
	}
	assert.Equal(t, 2, geminiAttempts)
}
