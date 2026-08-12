package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promptRecordingBackend answers with a marker unique to each provider and keeps
// every prompt it was handed, so a test can inspect what round 2 actually saw.
type promptRecordingBackend struct {
	mu      sync.Mutex
	prompts map[string][]string
}

func (b *promptRecordingBackend) Name() string { return "recording" }

func (b *promptRecordingBackend) freshExecutionPerRequest() bool { return true }

func (b *promptRecordingBackend) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	b.mu.Lock()
	if b.prompts == nil {
		b.prompts = make(map[string][]string)
	}
	b.prompts[req.Role] = append(b.prompts[req.Role], req.Prompt)
	b.mu.Unlock()

	if req.Role == "judge" {
		body, _ := json.Marshal(JudgeOutput{Recommendation: "proceed"})
		return &ProviderResponse{Provider: req.Provider, Output: string(body)}, nil
	}
	body, _ := json.Marshal(DebaterR1Output{
		CurrentState: markerFor(req.Provider),
		Ideas: []IdeaOutput{{
			Title:       "self inclusion",
			Description: markerFor(req.Provider),
			Rationale:   "observable behavior",
			Risks:       "none",
			Category:    "runtime",
		}},
	})
	return &ProviderResponse{Provider: req.Provider, Output: string(body)}, nil
}

func (b *promptRecordingBackend) promptsFor(role string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.prompts[role]...)
}

func markerFor(provider string) string {
	return fmt.Sprintf("MARKER-%s-ROUND1-CLAIM", strings.ToUpper(provider))
}

func runSelfInclusionPipeline(t *testing.T, providers []ProviderConfig) *promptRecordingBackend {
	t.Helper()
	backend := &promptRecordingBackend{}
	_, err := RunSubprocessPipeline(context.Background(), SubprocessPipelineConfig{
		Backend:   backend,
		Providers: providers,
		Topic:     "self inclusion contract",
		PromptData: PromptData{
			ProjectName: "autopus-adk", ProjectSummary: "contract test",
			TechStack: "Go", Topic: "self inclusion contract", MaxTurns: 5,
		},
		Rounds: 1,
		Judge:  ProviderConfig{Name: "judge", Binary: "judge"},
	})
	require.NoError(t, err)
	return backend
}

// Every debater must see the full anonymized round-1 set in round 2 — its own
// prior answer included.
//
// This looks like an echo chamber and it is tempting to "fix" by withholding a
// participant's own output. That was measured on 90 model-questions across two
// independent lineages: removing self-inclusion made results strictly worse
// (10 of 90 answers lost, McNemar p=0.0129). Self-inclusion is load-bearing;
// any change here has to beat that measurement first.
func TestSubprocessPipeline_Round2ShowsEachDebaterItsOwnRound1Answer(t *testing.T) {
	t.Parallel()

	providers := []ProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "codex", Binary: "codex"},
		{Name: "gemini", Binary: "agy"},
	}
	backend := runSelfInclusionPipeline(t, providers)

	round2 := backend.promptsFor("debater_r2")
	require.Len(t, round2, len(providers))

	for _, prompt := range round2 {
		for _, provider := range providers {
			assert.Contains(t, prompt, markerFor(provider.Name),
				"round 2 prompt must carry every round-1 answer, including the recipient's own")
		}
	}
}

// One prompt is broadcast to every debater. Identical prompts is the machine-
// checkable form of "nobody's own answer was filtered out": per-recipient
// tailoring is exactly how self-exclusion would be reintroduced.
func TestSubprocessPipeline_Round2BroadcastsOneIdenticalPrompt(t *testing.T) {
	t.Parallel()

	backend := runSelfInclusionPipeline(t, []ProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "codex", Binary: "codex"},
	})

	round2 := backend.promptsFor("debater_r2")
	require.Len(t, round2, 2)
	assert.Equal(t, round2[0], round2[1],
		"per-recipient round 2 prompts would mean somebody's own answer was withheld")
}
