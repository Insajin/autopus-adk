package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStructuredResponse_Numbered(t *testing.T) {
	t.Parallel()
	input := "1. First item\n2. Second item\n3. Third item"
	items, err := parseStructuredResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "First item", items[1])
	assert.Equal(t, "Second item", items[2])
	assert.Equal(t, "Third item", items[3])
}

func TestParseStructuredResponse_Paren(t *testing.T) {
	t.Parallel()
	input := "1) Alpha\n2) Beta"
	items, err := parseStructuredResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "Alpha", items[1])
	assert.Equal(t, "Beta", items[2])
}

func TestParseStructuredResponse_Bullet(t *testing.T) {
	t.Parallel()
	input := "- Apple\n- Banana\n- Cherry"
	items, err := parseStructuredResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "Apple", items[1])
	assert.Equal(t, "Banana", items[2])
}

func TestParseStructuredResponse_Empty(t *testing.T) {
	t.Parallel()
	_, err := parseStructuredResponse("no list items here, just plain text")
	assert.Error(t, err)
}

func TestMergeStructuredConsensus_AllAgree(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "p1", Output: "1. Use Go\n2. Write tests\n3. Keep it simple"},
		{Provider: "p2", Output: "1. Use Go\n2. Write tests\n3. Keep it simple"},
	}
	merged, summary := MergeStructuredConsensus(responses, 0.66)
	assert.NotEmpty(t, merged)
	assert.Contains(t, summary, "합의율")
	assert.Contains(t, merged, "합의된 내용")
}

func TestMergeStructuredConsensus_RequiresMatchingItemText(t *testing.T) {
	t.Parallel()

	responses := []ProviderResponse{
		{Provider: "p1", Output: "1. Use SQL\n2. Write tests"},
		{Provider: "p2", Output: "1. Use NoSQL\n2. Write tests"},
	}

	merged, _ := MergeStructuredConsensus(responses, 0.66)

	assert.NotContains(t, merged, "✓ 1. Use SQL")
	assert.NotContains(t, merged, "✓ 1. Use NoSQL")
	assert.Contains(t, merged, "△ 1.")
	assert.Contains(t, merged, "✓ 2. Write tests")
}

func TestMergeStructuredConsensus_FallbackOnUnparseable(t *testing.T) {
	t.Parallel()
	// One response has no list items → should fall back (return empty)
	responses := []ProviderResponse{
		{Provider: "p1", Output: "1. Use Go\n2. Write tests"},
		{Provider: "p2", Output: "just plain prose with no list"},
	}
	merged, _ := MergeStructuredConsensus(responses, 0.66)
	// Should return empty to signal fallback needed
	assert.Empty(t, merged)
}

func TestMergeStructuredConsensus_Empty(t *testing.T) {
	t.Parallel()
	merged, summary := MergeStructuredConsensus(nil, 0.66)
	assert.Empty(t, merged)
	assert.Empty(t, summary)
}

func TestBuildStructuredPromptPrefix(t *testing.T) {
	t.Parallel()
	prefix := buildStructuredPromptPrefix()
	assert.True(t, strings.Contains(prefix, "numbered list"))
}

func TestMergeConsensus_StructuredFirst(t *testing.T) {
	t.Parallel()
	// When responses are structured, MergeConsensus should use structured path
	responses := []ProviderResponse{
		{Provider: "p1", Output: "1. Item A\n2. Item B"},
		{Provider: "p2", Output: "1. Item A\n2. Item B"},
	}
	merged, summary := MergeConsensus(responses, 0.66)
	assert.NotEmpty(t, merged)
	assert.Contains(t, summary, "합의율")
}

func TestMergeConsensus_FallsBackToLineBased(t *testing.T) {
	t.Parallel()
	// Plain text responses → falls back to line-based
	responses := []ProviderResponse{
		{Provider: "p1", Output: "hello world"},
		{Provider: "p2", Output: "hello world"},
	}
	merged, summary := MergeConsensus(responses, 0.66)
	assert.NotEmpty(t, merged)
	assert.NotEmpty(t, summary)
}
