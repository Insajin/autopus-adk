package orchestra

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend implements ExecutionBackend for testing.
type mockBackend struct {
	name      string
	responses map[string]*ProviderResponse // keyed by provider name
	err       error
	mu        sync.Mutex
	requests  []ProviderRequest
}

func (m *mockBackend) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	m.mu.Lock()
	m.requests = append(m.requests, req)
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if resp, ok := m.responses[req.Provider]; ok {
		return resp, nil
	}
	// Default: return a valid debater/judge response based on role.
	return &ProviderResponse{
		Provider: req.Provider,
		Output:   defaultOutput(req.Role),
	}, nil
}

func (m *mockBackend) Name() string { return m.name }

func (m *mockBackend) Requests() []ProviderRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]ProviderRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

func defaultOutput(role string) string {
	switch role {
	case "judge":
		j := JudgeOutput{
			ConsensusAreas: []ConsensusArea{{Idea: "test", Participants: []string{"A"}, Significance: "ok"}},
			TopIdeas:       []RankedIdea{{Rank: 1, Title: "idea", Impact: 8, Confidence: 7, Ease: 6, Score: 3.36}},
			Recommendation: "proceed",
		}
		data, _ := json.Marshal(j)
		return string(data)
	default:
		d := DebaterR1Output{
			CurrentState: "good",
			Ideas:        []IdeaOutput{{Title: "idea1", Description: "d", Rationale: "r", Risks: "r", Category: "c"}},
			Assumptions:  []AssumptionOut{{Type: "value", Description: "d", RiskLevel: "low"}},
			HMWQuestions: []string{"how?"},
		}
		data, _ := json.Marshal(d)
		return string(data)
	}
}

func TestRunSubprocessPipeline_FastMode(t *testing.T) {
	t.Parallel()
	backend := &mockBackend{name: "mock"}
	cfg := SubprocessPipelineConfig{
		Backend:   backend,
		Providers: []ProviderConfig{{Name: "p1", Binary: "echo"}, {Name: "p2", Binary: "echo"}},
		Topic:     "test topic",
		PromptData: PromptData{
			ProjectName: "test", ProjectSummary: "test", TechStack: "Go",
			MustReadFiles: []string{"go.mod"}, Topic: "test topic", MaxTurns: 5,
		},
		Rounds: 0, // fast mode
		Judge:  ProviderConfig{Name: "judge", Binary: "echo"},
	}

	result, err := backend.Execute(context.Background(), ProviderRequest{Provider: "judge", Role: "judge"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Output)

	res, err := RunSubprocessPipeline(context.Background(), cfg)
	require.NoError(t, err)
	assert.Contains(t, res.Merged, "Orchestra Result")
	assert.Equal(t, StrategyDebate, res.Strategy)
}

func TestRunSubprocessPipeline_StandardMode(t *testing.T) {
	t.Parallel()
	backend := &mockBackend{name: "mock"}
	cfg := SubprocessPipelineConfig{
		Backend:   backend,
		Providers: []ProviderConfig{{Name: "p1", Binary: "echo"}},
		Topic:     "test topic",
		PromptData: PromptData{
			ProjectName: "test", ProjectSummary: "s", TechStack: "Go",
			MustReadFiles: []string{"go.mod"}, Topic: "test topic", MaxTurns: 5,
		},
		Rounds: 1,
		Judge:  ProviderConfig{Name: "judge", Binary: "echo"},
	}

	res, err := RunSubprocessPipeline(context.Background(), cfg)
	require.NoError(t, err)
	assert.Contains(t, res.Summary, "1 providers")
	assert.Contains(t, res.Summary, "2 rounds")
}

func TestRunSubprocessPipeline_NoProviders(t *testing.T) {
	t.Parallel()
	_, err := RunSubprocessPipeline(context.Background(), SubprocessPipelineConfig{
		Backend: &mockBackend{name: "mock"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no providers")
}

func TestRunSubprocessPipeline_NilBackend(t *testing.T) {
	t.Parallel()
	_, err := RunSubprocessPipeline(context.Background(), SubprocessPipelineConfig{
		Providers: []ProviderConfig{{Name: "p1"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backend is nil")
}

func TestRoundPresets(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, RoundPresets["fast"])
	assert.Equal(t, 1, RoundPresets["standard"])
	assert.Equal(t, 2, RoundPresets["deep"])
}

func TestRunSubprocessPipeline_ForwardsTimeoutsToRequests(t *testing.T) {
	t.Parallel()
	backend := &mockBackend{name: "mock"}
	cfg := SubprocessPipelineConfig{
		Backend: backend,
		Providers: []ProviderConfig{
			{Name: "p1", Binary: "echo"},
			{Name: "p2", Binary: "echo", ExecutionTimeout: 45 * time.Second},
		},
		Topic: "test topic",
		PromptData: PromptData{
			ProjectName: "test", ProjectSummary: "s", TechStack: "Go",
			MustReadFiles: []string{"go.mod"}, Topic: "test topic", MaxTurns: 5,
		},
		Rounds:         1,
		Judge:          ProviderConfig{Name: "judge", Binary: "echo", ExecutionTimeout: 90 * time.Second},
		TimeoutSeconds: 120,
	}

	_, err := RunSubprocessPipeline(context.Background(), cfg)
	require.NoError(t, err)

	requests := backend.Requests()
	require.Len(t, requests, 5)

	var judgeSeen bool
	for _, req := range requests {
		switch req.Provider {
		case "p1":
			assert.Equal(t, 120*time.Second, req.Timeout)
		case "p2":
			assert.Equal(t, 45*time.Second, req.Timeout)
		case "judge":
			judgeSeen = true
			assert.Equal(t, 90*time.Second, req.Timeout)
		}
	}
	assert.True(t, judgeSeen, "judge request must be recorded")
}
