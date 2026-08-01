package content_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

// TestTransformRuleForOMP_SynthesisContract pins the synthesis rules that keep
// omp discovery correct: the title wins over prose, a titleless body falls back
// to its first prose line, fenced code never supplies the text, the result is
// capped, and a rule that already carries any recognized key is left alone so
// its TTSR routing cannot change.
func TestTransformRuleForOMP_SynthesisContract(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("가나다라마 ", 60)
	tests := []struct {
		name string
		raw  string
		want any
	}{
		{
			name: "title wins over prose",
			raw:  "Lead prose line.\n\n# Real Title\n\nMore body.\n",
			want: map[string]any{"description": "Real Title"},
		},
		{
			name: "titleless body falls back to first prose line",
			raw:  "## Sub Heading\n\n- list item\n\n| a | b |\n\nThe first real sentence.\n",
			want: map[string]any{"description": "The first real sentence."},
		},
		{
			name: "fenced comment is not a title",
			raw:  "```sh\n# not-a-title\n```\n\nPlain lead line.\n",
			want: map[string]any{"description": "Plain lead line."},
		},
		{
			name: "inline markup is flattened",
			raw:  "# `Code` and **bold** and [link](http://example.com)\n\nBody.\n",
			want: map[string]any{"description": "Code and bold and link"},
		},
		{
			name: "long title is capped on a rune boundary",
			raw:  "# " + long + "\n\nBody.\n",
			want: nil, // asserted separately below
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := content.TransformRuleForOMP(tc.raw)
			require.NoError(t, err)
			keys, _ := decodeOMPRuleFrontmatter(t, out)
			require.NotNil(t, keys)

			if tc.want == nil {
				description, ok := keys["description"].(string)
				require.True(t, ok)
				assert.LessOrEqual(t, len([]rune(description)), 120,
					"a synthesized description is capped at 120 runes")
				assert.True(t, strings.HasPrefix(long, description),
					"the cap truncates the title instead of rewriting it, got %q", description)
				return
			}
			assert.Equal(t, tc.want, keys)
		})
	}
}

// TestTransformRuleForOMP_SynthesisSkipsRecognizedKeys guards the routing
// invariant behind the whole design: a rule carrying a trigger key stays in the
// TTSR engine, and gaining a description would move it into the always-listed
// <domain-rules> block instead.
func TestTransformRuleForOMP_SynthesisSkipsRecognizedKeys(t *testing.T) {
	t.Parallel()

	triggerOnly := "---\ncondition: git\\s+commit\nscope:\n  - tool:bash\n---\n\n# Trigger Rule\n\nBody.\n"
	out, err := content.TransformRuleForOMP(triggerOnly)
	require.NoError(t, err)

	keys, _ := decodeOMPRuleFrontmatter(t, out)
	require.NotNil(t, keys)
	assert.NotContains(t, keys, "description",
		"a trigger-carrying rule must not gain a description, which would reroute it out of TTSR")
	assert.Equal(t, `git\s+commit`, keys["condition"])
	assert.Equal(t, []any{"tool:bash"}, keys["scope"])
}

// TestTransformRuleForOMP_SynthesisDeterministic pins REQ-003 for the synthesis
// path specifically, so a regenerated rule never produces a spurious diff.
func TestTransformRuleForOMP_SynthesisDeterministic(t *testing.T) {
	t.Parallel()

	first, err := content.TransformRuleForOMP(categoryOnlyRule)
	require.NoError(t, err)
	assert.Contains(t, first, "description: Category Only")

	for i := 0; i < 8; i++ {
		next, err := content.TransformRuleForOMP(categoryOnlyRule)
		require.NoError(t, err)
		require.Equal(t, first, next, "repeated synthesis must emit identical bytes")
	}
}
