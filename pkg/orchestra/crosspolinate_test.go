package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrossPollinateBuilder_Anonymize(t *testing.T) {
	t.Parallel()
	cpb := NewCrossPollinateBuilder([]string{"claude", "codex", "gemini"})
	results := []ProviderResult{
		{Provider: "claude", Output: "Claude's ideas"},
		{Provider: "codex", Output: "Codex's analysis"},
		{Provider: "gemini", Output: "Gemini's thoughts"},
	}
	anon := cpb.Anonymize(results)
	require.Len(t, anon, 3)

	// Verify aliases are used, not real names.
	for _, a := range anon {
		assert.NotContains(t, a.Alias, "claude")
		assert.NotContains(t, a.Alias, "codex")
		assert.NotContains(t, a.Alias, "gemini")
		assert.Contains(t, a.Alias, "Debater")
	}
}

func TestCrossPollinateBuilder_ContentPreserved(t *testing.T) {
	t.Parallel()
	cpb := NewCrossPollinateBuilder([]string{"claude"})
	original := "This is a detailed analysis with specific recommendations about caching.\nIt spans multiple lines."
	results := []ProviderResult{{Provider: "claude", Output: original}}
	anon := cpb.Anonymize(results)
	require.Len(t, anon, 1)
	assert.Contains(t, anon[0].Output, "detailed analysis")
	assert.Contains(t, anon[0].Output, "multiple lines")
}

func TestCrossPollinateBuilder_ICEScoreStripped(t *testing.T) {
	t.Parallel()
	cpb := NewCrossPollinateBuilder([]string{"claude"})
	withICE := "Good ideas here.\n\n## ICE Scoring\n| Rank | Idea | Impact | Confidence | Ease | Score |\n| 1 | Cache | 8 | 7 | 6 | 3.36 |\n\nMore text."
	results := []ProviderResult{{Provider: "claude", Output: withICE}}
	anon := cpb.Anonymize(results)
	require.Len(t, anon, 1)
	assert.NotContains(t, anon[0].Output, "3.36")
	assert.Contains(t, anon[0].Output, "Good ideas")
	assert.Contains(t, anon[0].Output, "More text")
}

func TestCrossPollinateBuilder_AnonymizeForJudge(t *testing.T) {
	t.Parallel()
	cpb := NewCrossPollinateBuilder([]string{"claude", "codex"})
	r1 := []ProviderResult{
		{Provider: "claude", Output: "r1 claude"},
		{Provider: "codex", Output: "r1 codex"},
	}
	r2 := []ProviderResult{
		{Provider: "claude", Output: "r2 claude"},
		{Provider: "codex", Output: "r2 codex"},
	}
	judge := cpb.AnonymizeForJudge(r1, r2)
	require.Len(t, judge, 2)
	for _, j := range judge {
		assert.Contains(t, j.Alias, "Debater")
		assert.NotEmpty(t, j.Round1)
		assert.NotEmpty(t, j.Round2)
	}
}

func TestCrossPollinateBuilder_IdentityMap(t *testing.T) {
	t.Parallel()
	cpb := NewCrossPollinateBuilder([]string{"claude", "codex"})
	im := cpb.IdentityMap()
	assert.Equal(t, "claude", im["Debater A"])
	assert.Equal(t, "codex", im["Debater B"])
}

func TestStripICEScores_NoScores(t *testing.T) {
	t.Parallel()
	input := "Regular text without any scores."
	assert.Equal(t, input, stripICEScores(input))
}
