package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJudgeBuilder_Build(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)
	jb := NewJudgeBuilder(pb)

	data := PromptData{
		ProjectName:    "test-project",
		ProjectSummary: "A test project",
		TechStack:      "Go",
		MustReadFiles:  []string{"go.mod"},
		Topic:          "caching strategy",
		MaxTurns:       10,
	}
	results := []JudgeResult{
		{Alias: "Debater A", Round1: "ideas from A", Round2: "refined A"},
		{Alias: "Debater B", Round1: "ideas from B", Round2: "refined B"},
	}

	req, err := jb.Build(data, results)
	require.NoError(t, err)
	assert.Equal(t, "judge", req.Role)
	assert.Contains(t, req.Prompt, "caching strategy")
	assert.Contains(t, req.Prompt, "Debater A")
	assert.Contains(t, req.Prompt, "ICE")
}

func TestMergeSubprocessResults(t *testing.T) {
	t.Parallel()
	judge := &JudgeOutput{
		ConsensusAreas: []ConsensusArea{
			{Idea: "caching", Participants: []string{"A", "B"}, Significance: "perf"},
		},
		UniqueInsights: []UniqueInsight{
			{Idea: "novel approach", Proposer: "C", WhyMissed: "too complex"},
		},
		CrossRisks: []CrossRisk{
			{Risk: "memory pressure", Flaggers: []string{"A", "B"}, Severity: "high"},
		},
		TopIdeas: []RankedIdea{
			{Rank: 1, Title: "Redis cache", Impact: 9, Confidence: 8, Ease: 7, Score: 5.04},
		},
		Recommendation: "Implement Redis caching layer.",
	}
	idMap := map[string]string{"Debater A": "claude", "Debater B": "codex"}
	r1 := []ProviderResult{
		{Provider: "claude", Output: "claude r1"},
		{Provider: "codex", Output: "codex r1"},
	}
	r2 := []ProviderResult{
		{Provider: "claude", Output: "claude r2"},
		{Provider: "codex", Output: "codex r2"},
	}

	result := MergeSubprocessResults(judge, idMap, r1, r2)
	assert.Contains(t, result, "Orchestra Result")
	assert.Contains(t, result, "caching")
	assert.Contains(t, result, "Redis cache")
	assert.Contains(t, result, "5.04")
	assert.Contains(t, result, "claude")
	assert.Contains(t, result, "codex")
	assert.Contains(t, result, "Implement Redis")
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short", truncate("short", 10))
	assert.Equal(t, "12345...", truncate("1234567890", 5))
}
