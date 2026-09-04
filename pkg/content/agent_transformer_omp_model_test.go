package content_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformAgentForOMPWithModel_S2_EmitsRoleAndThinking(t *testing.T) {
	t.Parallel()

	src := ompAgentSource(t, "planner")
	out, err := content.TransformAgentForOMPWithModel(src, content.OMPAgentModelSelection{
		Model:    "@plan",
		Thinking: "xhigh",
	})
	require.NoError(t, err)
	fm, keys, _ := parseOMPAgentOutput(t, out)
	assert.Equal(t, "@plan", fm.Model)
	assert.Equal(t, "xhigh", keys["thinking"])
	assert.NotContains(t, out, "model: opus")
}

func TestTransformAgentForOMPWithModel_AcceptsNativeThinkingLevels(t *testing.T) {
	t.Parallel()

	src := ompAgentSource(t, "planner")
	for _, thinking := range []string{"off", "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto"} {
		out, err := content.TransformAgentForOMPWithModel(src, content.OMPAgentModelSelection{
			Model: "@plan", Thinking: thinking,
		})
		require.NoError(t, err, thinking)
		_, keys, _ := parseOMPAgentOutput(t, out)
		assert.Equal(t, thinking, keys["thinking"])
	}
}

func TestTransformAgentForOMP_WithoutOptIn_OmitsLegacyModelsToInheritParent(t *testing.T) {
	t.Parallel()

	for _, legacyModel := range []string{"fable", "opus", "sonnet", "haiku"} {
		src := content.AgentSource{
			Meta: content.AgentSourceMeta{
				Name: "legacy-agent", Description: "parent model inheritance fixture",
				Model: legacyModel, Tools: "Write, Read, WebSearch, WebFetch",
			},
			Body: "# Legacy Agent\n\nKeep this body.",
		}
		out := content.TransformAgentForOMP(src)
		fm, keys, _ := parseOMPAgentOutput(t, out)
		assert.Empty(t, fm.Model)
		assert.NotContains(t, keys, "model")
		assert.NotContains(t, keys, "thinking")
		assert.NotContains(t, out, "model: "+legacyModel)
	}
}

func TestTransformAgentForOMPWithModel_SecurityRejectsInjection(t *testing.T) {
	t.Parallel()

	src := ompAgentSource(t, "planner")
	for _, selection := range []content.OMPAgentModelSelection{
		{Model: "opus", Thinking: "high"},
		{Model: "@plan\ntools: [bash]", Thinking: "high"},
		{Model: "@plan", Thinking: "xhigh\ntools: [bash]"},
	} {
		out, err := content.TransformAgentForOMPWithModel(src, selection)
		require.Error(t, err)
		assert.Empty(t, out)
	}
}
