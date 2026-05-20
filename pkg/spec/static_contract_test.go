package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSpecContractAnalysis_FindsAuditAllowlistDrift(t *testing.T) {
	t.Parallel()

	specDir := writeContractSpec(t, map[string]string{
		"spec.md": `# SPEC-CONTRACT-001

## Retained-Field Allowlist

- ` + "`ts`" + ` — timestamp.
- ` + "`event_kind`" + ` — "session_open" | "session_close" | "trust_boundary_violation".
- ` + "`stance`" + ` — role stance.
- ` + "`redaction_status`" + ` — "redacted".

### Per-event-kind Non-null Column Table

| event_kind | event-specific non-null columns |
|------------|---------------------------------|
| ` + "`session_open`" + ` | none |
| ` + "`session_close`" + ` | none |
| ` + "`trust_boundary_violation`" + ` | ` + "`reason`" + ` |
`,
		"acceptance.md": `# Acceptance

Given a role emits an invalid stance
When the value is validated
Then an ` + "`event_kind=\"invalid_stance_value\"`" + ` row is appended with {attempted_value:"agree", expected_pool:["찬성","조건부","반대","blocker"]}.

Given a topic exceeds the call budget
When the 9th call is attempted
Then an ` + "`event_kind=\"cost_cap_exceeded\"`" + ` row is appended with {attempted_call_index:9, mitigation:"circuit_breaker_round_abort"}.
`,
	})

	findings, err := RunSpecContractAnalysis(specDir)
	require.NoError(t, err)
	require.Len(t, findings, 1)

	finding := findings[0]
	assert.Equal(t, "spec-static", finding.Provider)
	assert.Equal(t, FindingCategoryCompleteness, finding.Category)
	assert.Equal(t, "major", finding.Severity)
	assert.Equal(t, "spec-contract:retained-field-allowlist", finding.ScopeRef)
	assert.Contains(t, finding.Description, "missing_event_kind_values: cost_cap_exceeded, invalid_stance_value")
	assert.Contains(t, finding.Description, "missing_allowlist_columns: attempted_call_index, attempted_value, expected_pool, mitigation")
}

func TestRunSpecContractAnalysis_AllowsNestedCostMetaContainer(t *testing.T) {
	t.Parallel()

	specDir := writeContractSpec(t, map[string]string{
		"spec.md": `# SPEC-CONTRACT-002

## Retained-Field Allowlist

- ` + "`ts`" + ` — timestamp.
- ` + "`event_kind`" + ` — "session_open" | "session_close" | "trust_boundary_violation".
- ` + "`cost_meta`" + ` — nullable object. Cap-exceeded shape: {cost_cap_exceeded:true, attempted_call_index:number, mitigation:"circuit_breaker_round_abort"}.
- ` + "`redaction_status`" + ` — "redacted".

### Per-event-kind Non-null Column Table

| event_kind | event-specific non-null columns |
|------------|---------------------------------|
| ` + "`session_open`" + ` | none |
| ` + "`session_close`" + ` | ` + "`cost_meta`" + `. Cap-exceeded shape carries ` + "`cost_cap_exceeded`" + `, ` + "`attempted_call_index`" + `, and ` + "`mitigation`" + ` inside ` + "`cost_meta`" + `. |
| ` + "`trust_boundary_violation`" + ` | ` + "`reason`" + ` |
`,
		"acceptance.md": `# Acceptance

Given a topic exceeds the call budget
When the 9th call is attempted
Then an ` + "`event_kind=\"session_close\"`" + ` row is appended with ` + "`cost_meta.cost_cap_exceeded=true`" + `, ` + "`cost_meta.attempted_call_index=9`" + `, and ` + "`cost_meta.mitigation=\"circuit_breaker_round_abort\"`" + `.
`,
	})

	findings, err := RunSpecContractAnalysis(specDir)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func writeContractSpec(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	}
	return dir
}
