package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutputParser_ParseDebaterR2_Valid covers the previously-uncovered
// ParseDebaterR2 happy path and asserts the parsed nested fields.
func TestOutputParser_ParseDebaterR2_Valid(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{
		"acknowledgments": [{"source": "Debater A", "point": "caching", "why_strong": "reduces load"}],
		"integrated_ideas": [{"title": "hybrid", "description": "combine both"}],
		"risks": [{"description": "complexity", "severity": "medium", "mitigation": "phased rollout"}]
	}`
	out, err := op.ParseDebaterR2(input)
	require.NoError(t, err)
	require.Len(t, out.Acknowledgments, 1)
	assert.Equal(t, "Debater A", out.Acknowledgments[0].Source)
	assert.Equal(t, "reduces load", out.Acknowledgments[0].Why)
	require.Len(t, out.Risks, 1)
	assert.Equal(t, "medium", out.Risks[0].Severity)
	assert.Equal(t, "phased rollout", out.Risks[0].Mitigation)
}

// TestOutputParser_ParseDebaterR2_Empty verifies R2 accepts an empty object
// (no minimum-element constraint unlike R1).
func TestOutputParser_ParseDebaterR2_Empty(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	out, err := op.ParseDebaterR2("{}")
	require.NoError(t, err)
	assert.Empty(t, out.Acknowledgments)
	assert.Empty(t, out.IntegratedIdeas)
}

// TestOutputParser_ParseDebaterR2_NoJSON verifies the error branch when no JSON
// is present in the raw text.
func TestOutputParser_ParseDebaterR2_NoJSON(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	_, err := op.ParseDebaterR2("not json at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse debater_r2")
}

// TestOutputParser_ParseAny_AllRoles exercises every valid role branch of
// ParseAny and asserts the concrete returned type for each.
func TestOutputParser_ParseAny_AllRoles(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}

	r1, err := op.ParseAny(`{"ideas":[{"title":"x","description":"y"}]}`, "debater_r1")
	require.NoError(t, err)
	_, ok := r1.(*DebaterR1Output)
	assert.True(t, ok, "debater_r1 should return *DebaterR1Output")

	r2, err := op.ParseAny(`{"acknowledgments":[],"integrated_ideas":[],"risks":[]}`, "debater_r2")
	require.NoError(t, err)
	_, ok = r2.(*DebaterR2Output)
	assert.True(t, ok, "debater_r2 should return *DebaterR2Output")

	jd, err := op.ParseAny(`{"recommendation":"ship it"}`, "judge")
	require.NoError(t, err)
	jOut, ok := jd.(*JudgeOutput)
	require.True(t, ok)
	assert.Equal(t, "ship it", jOut.Recommendation)

	rv, err := op.ParseAny(`{"verdict":"PASS","summary":"looks good"}`, "reviewer")
	require.NoError(t, err)
	rvOut, ok := rv.(*ReviewerOutput)
	require.True(t, ok)
	assert.Equal(t, "PASS", rvOut.Verdict)
}

// TestOutputParser_ParseReviewer_FindingStatuses covers the finding-status
// validation branches, including the valid set and an invalid value.
func TestOutputParser_ParseReviewer_FindingStatuses(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}

	valid := `{"verdict":"REVISE","summary":"fix items","finding_statuses":[
		{"id":"F-1","status":"open"},
		{"id":"F-2","status":"resolved"},
		{"id":"F-3","status":"regressed"}
	]}`
	out, err := op.ParseReviewer(valid)
	require.NoError(t, err)
	require.Len(t, out.FindingStatus, 3)
	assert.Equal(t, "resolved", out.FindingStatus[1].Status)

	invalid := `{"verdict":"PASS","summary":"ok","finding_statuses":[{"id":"F-1","status":"bogus"}]}`
	_, err = op.ParseReviewer(invalid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid finding status")
}

// TestExtractJSON_MarkdownFence verifies extractJSON strips a ```json fenced block.
func TestExtractJSON_MarkdownFence(t *testing.T) {
	t.Parallel()
	raw := "Here is the result:\n```json\n{\"verdict\":\"PASS\",\"summary\":\"s\"}\n```\nDone."
	got := extractJSON(raw)
	assert.Contains(t, got, `"verdict"`)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(got), "{"))
}

// TestExtractJSON_BraceScan verifies extractJSON falls back to first-{ last-}
// scanning when the JSON is embedded in prose without fences.
func TestExtractJSON_BraceScan(t *testing.T) {
	t.Parallel()
	raw := `prefix text {"verdict":"REJECT","summary":"no"} trailing text`
	got := extractJSON(raw)
	assert.Equal(t, `{"verdict":"REJECT","summary":"no"}`, got)
}

// TestExtractJSON_Empty verifies empty / non-JSON inputs return empty string.
func TestExtractJSON_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", extractJSON(""))
	assert.Equal(t, "", extractJSON("   "))
	assert.Equal(t, "", extractJSON("no braces here just text"))
}

// TestExtractJSON_ClaudeEnvelope verifies the Claude stream-json result
// envelope is unwrapped to its inner JSON text.
func TestExtractJSON_ClaudeEnvelope(t *testing.T) {
	t.Parallel()
	raw := `{"type":"result","content":[{"type":"text","text":"{\"verdict\":\"PASS\",\"summary\":\"ok\"}"}]}`
	got := extractJSON(raw)
	assert.Equal(t, `{"verdict":"PASS","summary":"ok"}`, got)
}

// TestStripMarkdownJSON_PlainFence verifies a bare ``` fence (no json tag) is
// handled, and that a fence without a closing marker returns empty.
func TestStripMarkdownJSON_PlainFence(t *testing.T) {
	t.Parallel()
	got := stripMarkdownJSON("```\n{\"a\":1}\n```")
	assert.Equal(t, `{"a":1}`, got)

	assert.Equal(t, "", stripMarkdownJSON("no fence here"))
	assert.Equal(t, "", stripMarkdownJSON("```json no newline after fence"))
}
