package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVerdict_StructuredJSONDiscover(t *testing.T) {
	t.Parallel()

	output := `{"verdict":"REVISE","summary":"Needs work","findings":[{"severity":"major","category":"correctness","scope_ref":"REQ-001","description":"Missing timeout case","suggestion":"Add explicit timeout handling"}],"checklist":[{"id":"Q-CORR-01","status":"FAIL","reason":"timeout path missing"}]}`

	result := ParseVerdict("SPEC-JSON-001", output, "claude", 0, nil)
	assert.Equal(t, VerdictRevise, result.Verdict)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, FindingCategoryCorrectness, result.Findings[0].Category)
	assert.Equal(t, "REQ-001", result.Findings[0].ScopeRef)
	require.Len(t, result.ChecklistOutcomes, 1)
	assert.Equal(t, ChecklistStatusFail, result.ChecklistOutcomes[0].Status)
}

func TestParseVerdict_StructuredJSONVerify(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, Category: FindingCategoryCorrectness, Description: "Bug"},
	}
	output := `{"verdict":"REVISE","summary":"One finding remains","finding_statuses":[{"id":"F-001","status":"resolved","reason":"fixed"}],"findings":[{"severity":"critical","category":"security","scope_ref":"auth.go:42","description":"Token leak","suggestion":"Stop logging secrets"}]}`

	result := ParseVerdict("SPEC-JSON-002", output, "claude", 1, prior)
	require.Len(t, result.Findings, 2)
	assert.Equal(t, FindingStatusResolved, result.Findings[0].Status)
	assert.True(t, result.Findings[1].EscapeHatch)
	assert.Equal(t, FindingCategorySecurity, result.Findings[1].Category)
}
