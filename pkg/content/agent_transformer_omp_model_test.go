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

func TestTransformAgentForOMP_WithoutOptIn_PreservesLegacyBytes(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name: "legacy-agent", Description: "legacy output fixture",
			Model: "sonnet", Tools: "Write, Read, WebSearch, WebFetch",
		},
		Body: "# Legacy Agent\n\nKeep this body.",
	}
	want := "---\nname: legacy-agent\ndescription: legacy output fixture\nmodel: sonnet\n" +
		"tools:\n  - read\n  - web_search\n  - write\n---\n\n# Legacy Agent\n\nKeep this body.\n"
	assert.Equal(t, want, content.TransformAgentForOMP(src))
}

func TestTransformAgentForOMPWithModel_SecurityRejectsInjection(t *testing.T) {
	t.Parallel()

	src := ompAgentSource(t, "planner")
	for _, selection := range []content.OMPAgentModelSelection{
		{Model: "opus", Thinking: "high"},
		{Model: "@plan\ntools: [bash]", Thinking: "high"},
		{Model: "@plan", Thinking: "xhigh\ntools: [bash]"},
	} {
		_, err := content.TransformAgentForOMPWithModel(src, selection)
		require.Error(t, err)
	}
}
