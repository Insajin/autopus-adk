package content_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/content"
)

// conditionalDemoRule declares every frontmatter key the omp rule parser
// recognizes plus two keys outside that set (name, category).
const conditionalDemoRule = `---
name: conditional-demo
description: demo
globs:
  - "**/*.go"
alwaysApply: false
condition: tool:bash
astCondition: call:exec
scope:
  - tool:edit
interruptMode: prose-only
category: workflow
---

# Conditional Demo

Trigger body line.
`

// plainDemoRule declares no frontmatter at all.
const plainDemoRule = `# Plain Demo

Plain body line.
`

// categoryOnlyRule declares a single key outside the omp recognized set.
const categoryOnlyRule = `---
category: workflow
---

# Category Only

Category-only body line.
`

// splitOMPRuleOutput returns the emitted frontmatter text and body. An empty
// frontmatter string means the output carries no frontmatter block.
func splitOMPRuleOutput(t *testing.T, out string) (string, string) {
	t.Helper()

	if !strings.HasPrefix(out, "---\n") {
		return "", out
	}
	rest := strings.TrimPrefix(out, "---\n")
	end := strings.Index(rest, "\n---\n")
	require.GreaterOrEqual(t, end, 0, "an opened frontmatter fence must close")
	return rest[:end+1], strings.TrimLeft(rest[end+len("\n---\n"):], "\n")
}

func decodeOMPRuleFrontmatter(t *testing.T, out string) (map[string]any, string) {
	t.Helper()

	fmText, body := splitOMPRuleOutput(t, out)
	if fmText == "" {
		return nil, body
	}
	keys := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(fmText), &keys),
		"emitted frontmatter must be valid YAML for the omp rule parser")
	return keys, body
}

// TestTransformRuleForOMP_S2_RecognizedKeysPassThrough pins REQ-003: every
// recognized key survives with its source value and type, including the
// conditional trigger fields.
func TestTransformRuleForOMP_S2_RecognizedKeysPassThrough(t *testing.T) {
	t.Parallel()

	out, err := content.TransformRuleForOMP(conditionalDemoRule)
	require.NoError(t, err)

	keys, body := decodeOMPRuleFrontmatter(t, out)
	require.NotNil(t, keys, "a source declaring recognized keys must emit a frontmatter block")

	assert.Equal(t, "demo", keys["description"])
	assert.Equal(t, "tool:bash", keys["condition"])
	assert.Equal(t, "call:exec", keys["astCondition"])
	assert.Equal(t, "prose-only", keys["interruptMode"])
	assert.Equal(t, false, keys["alwaysApply"], "alwaysApply must stay a bool, not a string")
	assert.Equal(t, []any{"tool:edit"}, keys["scope"], "scope must carry exactly one entry")
	assert.Equal(t, []any{"**/*.go"}, keys["globs"])
	assert.Contains(t, body, "Trigger body line.")
}

// TestTransformRuleForOMP_S2_UnrecognizedKeysDropped pins the drop half of
// REQ-003.
func TestTransformRuleForOMP_S2_UnrecognizedKeysDropped(t *testing.T) {
	t.Parallel()

	out, err := content.TransformRuleForOMP(conditionalDemoRule)
	require.NoError(t, err)

	keys, _ := decodeOMPRuleFrontmatter(t, out)
	require.NotNil(t, keys)

	assert.NotContains(t, keys, "name")
	assert.NotContains(t, keys, "category")
	assert.Len(t, keys, 7, "only the seven recognized keys may survive")
}

// TestTransformRuleForOMP_S2_PlainSourceGainsSynthesizedDescription pins
// REQ-003 for a source that declares no frontmatter at all. Measured against
// omp 17.1.8, a rule reaches the session only when its frontmatter carries a
// description, so the transformer synthesizes one from the body title rather
// than emitting a bare body that lands on disk and is never discovered.
func TestTransformRuleForOMP_S2_PlainSourceGainsSynthesizedDescription(t *testing.T) {
	t.Parallel()

	out, err := content.TransformRuleForOMP(plainDemoRule)
	require.NoError(t, err)

	keys, body := decodeOMPRuleFrontmatter(t, out)
	require.NotNil(t, keys, "a discoverable rule must carry a frontmatter block")
	assert.Equal(t, map[string]any{"description": "Plain Demo"}, keys,
		"synthesis emits a description and nothing else")
	for _, key := range []string{"condition:", "scope:", "alwaysApply:"} {
		assert.NotContains(t, out, key,
			"synthesis must not invent a trigger key that would reroute the rule to TTSR")
	}
	assert.Equal(t, strings.TrimSpace(plainDemoRule), strings.TrimSpace(body),
		"the body stays byte-identical")
}

// TestTransformRuleForOMP_E1_UnrecognizedOnlySourceGainsDescription pins E1: a
// source whose only key is outside the recognized set drops that key, gains a
// synthesized description, and keeps its body byte-identical.
func TestTransformRuleForOMP_E1_UnrecognizedOnlySourceGainsDescription(t *testing.T) {
	t.Parallel()

	out, err := content.TransformRuleForOMP(categoryOnlyRule)
	require.NoError(t, err)

	keys, body := decodeOMPRuleFrontmatter(t, out)
	require.NotNil(t, keys, "a discoverable rule must carry a frontmatter block")
	assert.Equal(t, map[string]any{"description": "Category Only"}, keys)
	assert.NotContains(t, out, "category:", "the unrecognized source key stays dropped")

	wantBody := "# Category Only\n\nCategory-only body line."
	assert.Equal(t, wantBody, strings.TrimSpace(body))
}

// TestTransformRuleForOMP_REQ004_EmptyBodyRejected pins REQ-004: the omp rule
// validator rejects an empty rule, so the transformer must fail closed instead
// of emitting one.
func TestTransformRuleForOMP_REQ004_EmptyBodyRejected(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"frontmatter only": "---\ndescription: demo\n---\n",
		"whitespace body":  "---\ndescription: demo\n---\n\n   \n\t\n",
		"empty input":      "",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := content.TransformRuleForOMP(raw)
			require.Error(t, err, "an empty body must not be emitted")
			assert.Empty(t, out)
		})
	}
}

// TestTransformRuleForOMP_REQ003_Deterministic guards against map-iteration
// nondeterminism leaking into generated rule bytes, which would make every
// regeneration a spurious diff.
func TestTransformRuleForOMP_REQ003_Deterministic(t *testing.T) {
	t.Parallel()

	first, err := content.TransformRuleForOMP(conditionalDemoRule)
	require.NoError(t, err)

	for i := 0; i < 8; i++ {
		next, err := content.TransformRuleForOMP(conditionalDemoRule)
		require.NoError(t, err)
		require.Equal(t, first, next, "repeated transforms must emit identical bytes")
	}
}
