package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRebuttalPrompt(t *testing.T) {
	t.Parallel()
	others := []ProviderResponse{
		{Provider: "alice", Output: "I think X"},
		{Provider: "bob", Output: "I think Y"},
	}
	prompt := buildRebuttalPrompt("original topic", others, 2)
	assert.Contains(t, prompt, "original topic")
	assert.Contains(t, prompt, "alice")
	assert.Contains(t, prompt, "I think X")
	assert.Contains(t, prompt, "bob")
	assert.Contains(t, prompt, "rebuttal")
}

// TestBuildRebuttalPrompt_TopicIsolation verifies buildRebuttalPrompt does NOT add isolation prefix.
func TestBuildRebuttalPrompt_TopicIsolation(t *testing.T) {
	t.Parallel()
	others := []ProviderResponse{
		{Provider: "alice", Output: "I think X"},
	}
	prompt := buildRebuttalPrompt("topic", others, 2)
	// buildRebuttalPrompt itself must NOT contain isolation prefix — caller adds it
	assert.NotContains(t, prompt, "IMPORTANT: Discuss ONLY")
	assert.NotContains(t, prompt, topicIsolationInstruction)
}

// TestBuildRebuttalPrompt_Summarization verifies round-based truncation.
func TestBuildRebuttalPrompt_Summarization(t *testing.T) {
	t.Parallel()
	longOutput := strings.Repeat("x", 2000)
	others := []ProviderResponse{
		{Provider: "alice", Output: longOutput},
	}

	// Round 2 — full output preserved
	promptR2 := buildRebuttalPrompt("topic", others, 2)
	assert.Contains(t, promptR2, longOutput, "round 2 should preserve full output")
	assert.NotContains(t, promptR2, "[...truncated]")

	// Round 3 — truncated to 500 chars
	promptR3 := buildRebuttalPrompt("topic", others, 3)
	assert.NotContains(t, promptR3, longOutput, "round 3 should truncate long output")
	assert.Contains(t, promptR3, "[...truncated]")
	// Verify the truncated output is 500 chars + "[...truncated]"
	assert.Contains(t, promptR3, longOutput[:500])
}

func TestBuildJudgmentPrompt(t *testing.T) {
	t.Parallel()
	args := []ProviderResponse{
		{Provider: "alice", Output: "Argument A"},
		{Provider: "bob", Output: "Argument B"},
	}
	prompt := buildJudgmentPrompt("test topic", args)
	assert.Contains(t, prompt, "test topic")
	assert.Contains(t, prompt, "alice")
	assert.Contains(t, prompt, "Argument A")
	assert.Contains(t, prompt, "judge")
}

func TestFindOrBuildJudgeConfig_Found(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		JudgeProvider: "myJudge",
		Providers: []ProviderConfig{
			{Name: "p1", Binary: "cat"},
			{Name: "myJudge", Binary: "/usr/bin/judge"},
		},
	}
	jc := findOrBuildJudgeConfig(cfg)
	assert.Equal(t, "/usr/bin/judge", jc.Binary)
}

func TestFindOrBuildJudgeConfig_NotFound(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		JudgeProvider: "judge",
		Providers: []ProviderConfig{
			{Name: "p1", Binary: "cat"},
		},
	}
	jc := findOrBuildJudgeConfig(cfg)
	assert.Equal(t, "judge", jc.Name)
	assert.Equal(t, "judge", jc.Binary)
}

func TestBuildDebateMerged_NoJudge(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "alice", Output: "Alice output"},
		{Provider: "bob", Output: "Bob output"},
	}
	cfg := OrchestraConfig{JudgeProvider: ""}
	merged, summary := buildDebateMerged(responses, cfg)
	assert.NotEmpty(t, merged)
	assert.Contains(t, summary, "판정")
	assert.Contains(t, summary, "없음")
}

func TestBuildDebateMerged_WithJudge(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "alice", Output: "Alice output"},
		{Provider: "bob", Output: "Bob output"},
		{Provider: "judge (judge)", Output: "Final verdict: Alice wins"},
	}
	cfg := OrchestraConfig{JudgeProvider: "judge"}
	merged, summary := buildDebateMerged(responses, cfg)
	assert.NotEmpty(t, merged)
	assert.Contains(t, summary, "judge")
	// Judge verdict should be reflected
	assert.True(t, strings.Contains(summary, "verdict") || strings.Contains(summary, "판정"))
}

func TestBuildDebateMerged_Empty(t *testing.T) {
	t.Parallel()
	_, summary := buildDebateMerged(nil, OrchestraConfig{})
	assert.Contains(t, summary, "결과 없음")
}
