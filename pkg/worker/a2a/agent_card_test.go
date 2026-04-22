package a2a

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCardBuilder_BasicBuild(t *testing.T) {
	t.Parallel()

	card := NewCardBuilder("my-worker", "https://api.example.com").Build()

	assert.Equal(t, "my-worker", card.Name)
	assert.Equal(t, "https://api.example.com", card.URL)
	assert.Equal(t, "Autopus ADK Worker", card.Description)
	assert.Equal(t, []string{"build_change"}, card.ExecutionLanes)
	assert.Equal(t, DefaultCapabilities(), card.Capabilities)
	assert.Equal(t, []string{"text"}, card.SupportedInputModes)
}

func TestCardBuilder_WithVersion(t *testing.T) {
	t.Parallel()

	card := NewCardBuilder("worker", "https://api.example.com").
		WithVersion("1.2.3").
		Build()

	assert.Equal(t, "Autopus ADK Worker v1.2.3", card.Description)
}

func TestCardBuilder_WithProviders_KnownProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider string
		expected []string
	}{
		{"claude", "claude", []string{"analysis", "coding", "review"}},
		{"codex", "codex", []string{"coding", "generation"}},
		{"gemini", "gemini", []string{"analysis", "coding", "search"}},
		{"opencode", "opencode", []string{"coding"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			card := NewCardBuilder("w", "http://x").
				WithProviders([]string{tc.provider}).
				Build()
			assert.Equal(t, tc.expected, card.Skills)
		})
	}
}

func TestCardBuilder_WithProviders_UnknownProvider(t *testing.T) {
	t.Parallel()

	card := NewCardBuilder("w", "http://x").
		WithProviders([]string{"unknown-llm"}).
		Build()

	assert.Equal(t, []string{"coding"}, card.Skills)
}

func TestCardBuilder_SkillDeduplication(t *testing.T) {
	t.Parallel()

	// claude has "coding", codex also has "coding" — should appear once.
	card := NewCardBuilder("w", "http://x").
		WithProviders([]string{"claude", "codex", "gemini"}).
		Build()

	// Verify no duplicates: sorted unique set.
	seen := make(map[string]bool)
	for _, s := range card.Skills {
		assert.False(t, seen[s], "duplicate skill: %s", s)
		seen[s] = true
	}

	// Expected skills: analysis, coding, generation, review, search (sorted).
	assert.Equal(t, []string{"analysis", "coding", "generation", "review", "search"}, card.Skills)
}

func TestCardBuilder_NoProviders(t *testing.T) {
	t.Parallel()

	card := NewCardBuilder("w", "http://x").
		WithProviders([]string{}).
		Build()

	assert.Empty(t, card.Skills)
}

func TestCardBuilder_WithExecutionLanes(t *testing.T) {
	t.Parallel()

	card := NewCardBuilder("w", "http://x").
		WithExecutionLanes([]string{"prod_observe", "build_change", "prod_observe"}).
		Build()

	assert.Equal(t, []string{"build_change", "prod_observe"}, card.ExecutionLanes)
}

func TestParseRegistrationResponse_Success(t *testing.T) {
	t.Parallel()

	data := []byte(`{"success": true, "worker_id": "w-123"}`)
	result, err := ParseRegistrationResponse(data)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "w-123", result.WorkerID)
	assert.Empty(t, result.Error)
}

func TestParseRegistrationResponse_Error(t *testing.T) {
	t.Parallel()

	data := []byte(`{"success": false, "error": "quota exceeded"}`)
	result, err := ParseRegistrationResponse(data)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "quota exceeded", result.Error)
}

func TestParseRegistrationResponse_MalformedJSON(t *testing.T) {
	t.Parallel()

	data := []byte(`{not valid json`)
	result, err := ParseRegistrationResponse(data)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "parse registration response")
}
