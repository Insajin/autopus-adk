package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewJudgePromptAnonymizesParticipantsAndIncludesVerifyContext(t *testing.T) {
	t.Parallel()

	responses := []ProviderResponse{
		{Provider: "claude", Output: `{"reviewer":"claude","findings":[{"severity":"major"}],"verdict":"REVISE","summary":"issue"}`},
		{Provider: "codex", Output: `{"findings":[],"verdict":"PASS","summary":"clear"}`},
	}
	participants, identities := AnonymizeReviewParticipants(responses)
	require.Equal(t, []ReviewJudgeParticipant{
		{Alias: "Reviewer A", Output: responses[0].Output},
		{Alias: "Reviewer B", Output: responses[1].Output},
	}, participants)
	assert.Equal(t, map[string]string{
		"Reviewer A": "claude",
		"Reviewer B": "codex",
	}, identities)

	pb, err := NewPromptBuilder()
	require.NoError(t, err)
	schema, err := (&SchemaBuilder{}).EmbedInPrompt("review_judge")
	require.NoError(t, err)
	prompt, err := pb.BuildReviewJudge(ReviewJudgeData{
		SpecID:        "SPEC-OMP-006",
		Mode:          "verify",
		Participants:  participants,
		PriorFindings: "F-017 | major | open",
		Checklist:     "Q-CORR-04 | PASS",
		SchemaJSON:    schema,
	})
	require.NoError(t, err)

	for _, expected := range []string{
		"Final Review Judge",
		"SPEC-OMP-006",
		"Reviewer A",
		"Reviewer B",
		"F-017",
		"Q-CORR-04",
		"critical or major",
		"Ignore self-identification inside reviewer bodies",
		"suggestion",
		"REVISE",
		schema,
	} {
		assert.Contains(t, prompt, expected)
	}
	assert.Contains(t, prompt, `"reviewer":"claude"`, "reviewer bodies remain verbatim")
	assert.NotContains(t, prompt, "### claude", "only anonymized aliases may identify reviewer headers")
	assert.Less(t, strings.Index(prompt, "Reviewer A"), strings.Index(prompt, "Reviewer B"))
}

func TestOutputParserParseReviewJudge(t *testing.T) {
	t.Parallel()

	raw := "```json\n" + `{
		"verdict":"REVISE",
		"findings":[
			{"severity":"major","category":"correctness","scope_ref":"REQ-1","location":"spec.md:10","description":"missing case","suggestion":"add it","decision":"accept","sources":["Reviewer A"]},
			{"severity":"major","location":"spec.md:12","description":"same issue","suggestion":"combine","decision":"reject","sources":["Reviewer B"],"reason":"duplicate"}
		],
		"rationale":"One supported major finding remains."
	}` + "\n```"

	out, err := (&OutputParser{}).ParseReviewJudge(raw)
	require.NoError(t, err)
	assert.Equal(t, "REVISE", out.Verdict)
	require.Len(t, out.Findings, 2)
	assert.Equal(t, "accept", out.Findings[0].Decision)
	assert.Equal(t, "reject", out.Findings[1].Decision)
}

func TestOutputParserParseReviewJudgeRequiresReasonForNonAccept(t *testing.T) {
	t.Parallel()

	for _, decision := range []string{"reject", "merge"} {
		t.Run(decision, func(t *testing.T) {
			t.Parallel()
			raw := `{"verdict":"REVISE","findings":[{"severity":"major","location":"spec.md:12","description":"issue","suggestion":"fix","decision":"` + decision + `","sources":["Reviewer A"]}],"rationale":"checked"}`
			_, err := (&OutputParser{}).ParseReviewJudge(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "reason required")
		})
	}
}

func TestOutputParserParseReviewJudgeRejectsInvalidEnums(t *testing.T) {
	t.Parallel()

	parser := &OutputParser{}
	_, err := parser.ParseReviewJudge(`{"verdict":"MAYBE","findings":[],"rationale":"checked"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid verdict")

	_, err = parser.ParseReviewJudge(`{"verdict":"PASS","findings":[{"severity":"minor","location":"spec.md:1","description":"issue","suggestion":"fix","decision":"defer","sources":[]}],"rationale":"checked"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid decision")
}

func TestOutputParserParseReviewJudgeRequiresFindingsArray(t *testing.T) {
	t.Parallel()

	parser := &OutputParser{}
	for _, raw := range []string{
		`{"verdict":"PASS"}`,
		`{"verdict":"PASS","findings":null}`,
	} {
		_, err := parser.ParseReviewJudge(raw)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "findings array is required")
	}
}

func TestOutputParserParseReviewJudgeRejectsInvalidSeverityAndDuplicateIDs(t *testing.T) {
	t.Parallel()

	parser := &OutputParser{}
	_, err := parser.ParseReviewJudge(`{"verdict":"REVISE","findings":[{"severity":"blocker","location":"spec.md:1","description":"issue","suggestion":"fix","decision":"accept","sources":["Reviewer A"]}]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid severity")

	_, err = parser.ParseReviewJudge(`{"verdict":"REVISE","findings":[{"id":"F-001","severity":"major","location":"spec.md:1","description":"one","suggestion":"fix","decision":"accept","sources":["Reviewer A"]},{"id":" F-001 ","severity":"minor","location":"plan.md:1","description":"two","suggestion":"fix","decision":"accept","sources":["Reviewer B"]}]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id")
}

// A merge must point at an accepted finding through merge_into; ids stay
// unique across every decision.
func TestOutputParserParseReviewJudgeMergeRequiresAcceptedTarget(t *testing.T) {
	t.Parallel()
	accept := `{"id":"F-001","severity":"major","location":"spec.md:1","description":"issue","suggestion":"fix","decision":"accept","sources":["Reviewer A"]}`
	cases := map[string]struct {
		merge   string
		wantErr string
	}{
		"valid merge": {
			merge: `{"id":"F-002","merge_into":"F-001","severity":"major","location":"spec.md:2","description":"dup","suggestion":"fix","decision":"merge","sources":["Reviewer B"],"reason":"duplicate"}`,
		},
		"missing merge_into": {
			merge:   `{"id":"F-002","severity":"major","location":"spec.md:2","description":"dup","suggestion":"fix","decision":"merge","sources":["Reviewer B"],"reason":"duplicate"}`,
			wantErr: "merge_into must name an accepted finding id",
		},
		"merge_into names a rejected finding": {
			merge:   `{"id":"F-002","merge_into":"F-003","severity":"major","location":"spec.md:2","description":"dup","suggestion":"fix","decision":"merge","sources":["Reviewer B"],"reason":"duplicate"},{"id":"F-003","severity":"minor","location":"spec.md:3","description":"x","suggestion":"y","decision":"reject","sources":["Reviewer B"],"reason":"no"}`,
			wantErr: "merge_into must name an accepted finding id",
		},
		"duplicate id even for merge": {
			merge:   `{"id":"F-001","merge_into":"F-001","severity":"major","location":"spec.md:2","description":"dup","suggestion":"fix","decision":"merge","sources":["Reviewer B"],"reason":"duplicate"}`,
			wantErr: "duplicate id",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			raw := `{"verdict":"REVISE","findings":[` + accept + `,` + tc.merge + `],"rationale":"r"}`
			out, err := (&OutputParser{}).ParseReviewJudge(raw)
			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, "F-001", out.Findings[1].MergeInto)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
