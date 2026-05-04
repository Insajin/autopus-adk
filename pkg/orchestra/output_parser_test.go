package orchestra

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputParser_ParseDebaterR1_Valid(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"current_state":"good","ideas":[{"title":"idea1","description":"d","rationale":"r","risks":"r","category":"architecture"}],"assumptions":[{"type":"value","description":"d","risk_level":"high"}],"hmw_questions":["q1"]}`
	out, err := op.ParseDebaterR1(input)
	require.NoError(t, err)
	assert.Equal(t, "good", out.CurrentState)
	assert.Len(t, out.Ideas, 1)
	assert.Equal(t, "idea1", out.Ideas[0].Title)
}

func TestOutputParser_ParseDebaterR1_NoIdeas(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"current_state":"ok","ideas":[],"assumptions":[],"hmw_questions":[]}`
	_, err := op.ParseDebaterR1(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1 idea")
}

func TestOutputParser_ParseJudge_Valid(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	judge := JudgeOutput{
		ConsensusAreas: []ConsensusArea{{Idea: "cache", Participants: []string{"A"}, Significance: "perf"}},
		TopIdeas:       []RankedIdea{{Rank: 1, Title: "t", Impact: 8, Confidence: 7, Ease: 6, Score: 3.36}},
		Recommendation: "proceed with caching",
	}
	data, _ := json.Marshal(judge)
	out, err := op.ParseJudge(string(data))
	require.NoError(t, err)
	assert.Equal(t, "proceed with caching", out.Recommendation)
}

func TestOutputParser_ParseJudge_NoRecommendation(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"consensus_areas":[],"unique_insights":[],"cross_risks":[],"top_ideas":[],"recommendation":""}`
	_, err := op.ParseJudge(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recommendation required")
}

func TestOutputParser_ParseReviewer_AllVerdicts(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	for _, v := range []string{"PASS", "REVISE", "REJECT"} {
		input := `{"findings":[],"verdict":"` + v + `","summary":"ok","checklist":[{"id":"Q-1","status":"PASS"}],"finding_statuses":[{"id":"F-001","status":"resolved"}]}`
		out, err := op.ParseReviewer(input)
		require.NoError(t, err, "verdict=%s", v)
		assert.Equal(t, v, out.Verdict)
	}
}

func TestOutputParser_ParseReviewer_InvalidVerdict(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"findings":[],"verdict":"MAYBE","summary":"ok"}`
	_, err := op.ParseReviewer(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid verdict")
}

func TestOutputParser_ParseReviewer_RequiresSummary(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"findings":[],"verdict":"PASS","summary":""}`
	_, err := op.ParseReviewer(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary required")
}

func TestOutputParser_ParseReviewer_InvalidChecklistStatus(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"findings":[],"verdict":"PASS","summary":"ok","checklist":[{"id":"Q-1","status":"MAYBE"}]}`
	_, err := op.ParseReviewer(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid checklist status")
}

func TestOutputParser_ParseReviewer_AcceptsNAChecklistStatus(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `{"findings":[],"verdict":"PASS","summary":"ok","checklist":[{"id":"Q-SEC-01","status":"N/A"},{"id":"Q-CORR-01","status":"PASS"}]}`
	out, err := op.ParseReviewer(input)
	require.NoError(t, err)
	assert.Equal(t, "PASS", out.Verdict)
	require.Len(t, out.Checklist, 2)
	assert.Equal(t, "N/A", out.Checklist[0].Status)
}

func TestOutputParser_MarkdownWrapped(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := "Some preamble\n```json\n" + `{"current_state":"s","ideas":[{"title":"t","description":"d","rationale":"r","risks":"r","category":"c"}],"assumptions":[],"hmw_questions":[]}` + "\n```\nSome suffix"
	out, err := op.ParseDebaterR1(input)
	require.NoError(t, err)
	assert.Equal(t, "s", out.CurrentState)
}

func TestOutputParser_ClaudeEnvelope(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	inner := `{"current_state":"s","ideas":[{"title":"t","description":"d","rationale":"r","risks":"r","category":"c"}],"assumptions":[],"hmw_questions":[]}`
	// Build a valid envelope where text is a proper JSON string value.
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type envelope struct {
		Type    string    `json:"type"`
		Content []content `json:"content"`
	}
	env := envelope{Type: "result", Content: []content{{Type: "text", Text: inner}}}
	data, _ := json.Marshal(env)
	out, err := op.ParseDebaterR1(string(data))
	require.NoError(t, err)
	assert.Equal(t, "s", out.CurrentState)
}

func TestOutputParser_EmptyInput(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	_, err := op.ParseDebaterR1("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON found")
}

func TestOutputParser_ParseAny_UnknownRole(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	_, err := op.ParseAny("{}", "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown role")
}

func TestOutputParser_PartialJSON(t *testing.T) {
	t.Parallel()
	op := &OutputParser{}
	input := `Here is my analysis: {"findings":[],"verdict":"PASS","summary":"all good"} end of response`
	out, err := op.ParseReviewer(input)
	require.NoError(t, err)
	assert.Equal(t, "PASS", out.Verdict)
}
