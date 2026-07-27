package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateScenarios_AlphanumericRefsRoundTripWithoutFieldBleed(t *testing.T) {
	t.Parallel()

	content := scenarioValidationDocument(
		validScenarioBlock("S1", "numeric", "echo numeric", "active", "exit_code(0)"),
		validScenarioBlock("S15A", "alpha-suffix", "echo alpha", "active", "exit_code(0)"),
		validScenarioBlock("S-CANARY-1", "canary", "echo canary", "active", "exit_code(0)"),
	)

	report, err := ValidateScenarios([]byte(content))
	require.NoError(t, err)
	require.NotNil(t, report)
	require.NotNil(t, report.ScenarioSet)
	require.Len(t, report.ScenarioSet.Scenarios, 3)

	assert.Equal(t, []string{"S1", "S15A", "S-CANARY-1"}, []string{
		report.ScenarioSet.Scenarios[0].Ref,
		report.ScenarioSet.Scenarios[1].Ref,
		report.ScenarioSet.Scenarios[2].Ref,
	})
	assert.Equal(t, []int{1, 0, 0}, []int{
		report.ScenarioSet.Scenarios[0].Number,
		report.ScenarioSet.Scenarios[1].Number,
		report.ScenarioSet.Scenarios[2].Number,
	})
	assert.Equal(t, []string{"echo numeric", "echo alpha", "echo canary"}, []string{
		report.ScenarioSet.Scenarios[0].Command,
		report.ScenarioSet.Scenarios[1].Command,
		report.ScenarioSet.Scenarios[2].Command,
	})

	rendered, err := RenderScenarios(report.ScenarioSet)
	require.NoError(t, err)
	roundTrip, err := ParseScenarios(rendered)
	require.NoError(t, err)
	require.Len(t, roundTrip.Scenarios, 3)
	assert.Equal(t, []string{"S1", "S15A", "S-CANARY-1"}, []string{
		roundTrip.Scenarios[0].DisplayRef(),
		roundTrip.Scenarios[1].DisplayRef(),
		roundTrip.Scenarios[2].DisplayRef(),
	})
}

func TestValidateScenarios_MalformedHeaderDoesNotOverwritePreviousScenario(t *testing.T) {
	t.Parallel()

	content := scenarioValidationDocument(
		validScenarioBlock("S1", "safe", "echo safe", "active", "exit_code(0)"),
		`### S!: malformed — Must not bleed
- **Command**: touch should-not-run
- **Verify**: unknown()
- **Status**: active
`,
	)

	report, err := ValidateScenarios([]byte(content))
	require.NoError(t, err)
	require.NotNil(t, report)
	require.NotNil(t, report.ScenarioSet)
	require.Len(t, report.ScenarioSet.Scenarios, 1)
	assert.Equal(t, "S1", report.ScenarioSet.Scenarios[0].DisplayRef())
	assert.Equal(t, "echo safe", report.ScenarioSet.Scenarios[0].Command)
	assert.Equal(t, []string{"exit_code(0)"}, report.ScenarioSet.Scenarios[0].Verify)

	require.Len(t, report.Issues, 1)
	assert.Equal(t, "scenario_malformed_header", report.Issues[0].Code)
	assert.Equal(t, "S!", report.Issues[0].ScenarioRef)
	assert.Equal(t, "Header", report.Issues[0].Field)
	assert.Positive(t, report.Issues[0].Line)
}

func TestValidateScenarios_EmitsExactStableReasonCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		blocks    []string
		wantCode  string
		wantRef   string
		wantField string
	}{
		{
			name: "missing status",
			blocks: []string{`### S1: missing-status — Missing status
- **Command**: echo ok
- **Verify**: exit_code(0)
`},
			wantCode: "scenario_missing_status", wantRef: "S1", wantField: "Status",
		},
		{
			name: "invalid status",
			blocks: []string{
				validScenarioBlock("S2", "invalid-status", "echo ok", "pending", "exit_code(0)"),
			},
			wantCode: "scenario_invalid_status", wantRef: "S2", wantField: "Status",
		},
		{
			name: "missing command",
			blocks: []string{`### S3: missing-command — Missing command
- **Verify**: exit_code(0)
- **Status**: active
`},
			wantCode: "scenario_missing_command", wantRef: "S3", wantField: "Command",
		},
		{
			name: "missing verify",
			blocks: []string{`### S4: missing-verify — Missing verify
- **Command**: echo ok
- **Status**: active
`},
			wantCode: "scenario_missing_verify", wantRef: "S4", wantField: "Verify",
		},
		{
			name: "unsupported verify",
			blocks: []string{
				validScenarioBlock("S5", "unsupported", "echo ok", "active", "unknown_check()"),
			},
			wantCode: "scenario_unsupported_verify", wantRef: "S5", wantField: "Verify",
		},
		{
			name: "duplicate ref",
			blocks: []string{
				validScenarioBlock("S6", "first", "echo first", "active", "exit_code(0)"),
				validScenarioBlock("S6", "second", "echo second", "active", "exit_code(0)"),
			},
			wantCode: "scenario_duplicate_ref", wantRef: "S6", wantField: "Ref",
		},
		{
			name: "duplicate id",
			blocks: []string{
				validScenarioBlock("S7", "same-id", "echo first", "active", "exit_code(0)"),
				validScenarioBlock("S8", "same-id", "echo second", "active", "exit_code(0)"),
			},
			wantCode: "scenario_duplicate_id", wantRef: "S8", wantField: "ID",
		},
		{
			name: "duplicate field",
			blocks: []string{`### S9: duplicate-command — Duplicate command
- **Command**: echo first
- **Command**: echo second
- **Verify**: exit_code(0)
- **Status**: active
`},
			wantCode: "scenario_duplicate_field", wantRef: "S9", wantField: "Command",
		},
		{
			name: "unknown field",
			blocks: []string{`### S10: unknown-field — Unknown field
- **Command**: echo ok
- **Verify**: exit_code(0)
- **Status**: active
- **Mystery**: value
`},
			wantCode: "scenario_unknown_field", wantRef: "S10", wantField: "Mystery",
		},
		{
			name: "malformed header",
			blocks: []string{`### S?: malformed — Malformed header
- **Command**: echo nope
- **Verify**: exit_code(0)
- **Status**: active
`},
			wantCode: "scenario_malformed_header", wantRef: "S?", wantField: "Header",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, err := ValidateScenarios([]byte(scenarioValidationDocument(tt.blocks...)))
			require.NoError(t, err)
			require.NotNil(t, report)
			require.Len(t, report.Issues, 1)
			assert.Equal(t, tt.wantCode, report.Issues[0].Code)
			assert.Equal(t, tt.wantRef, report.Issues[0].ScenarioRef)
			assert.Equal(t, tt.wantField, report.Issues[0].Field)
			assert.Positive(t, report.Issues[0].Line)
		})
	}
}

func TestValidateScenarios_OnlyExplicitValidActiveScenariosAreRunnable(t *testing.T) {
	t.Parallel()

	content := scenarioValidationDocument(
		validScenarioBlock("S1", "active", "echo active", "active", "exit_code(0)"),
		validScenarioBlock("S2", "deprecated", "echo deprecated", "deprecated", "exit_code(0)"),
		validScenarioBlock("S3", "skip", "echo skip", "skip", "exit_code(0)"),
		validScenarioBlock("S4", "reference", "echo reference", "reference", "exit_code(0)"),
		`### S5: implicit-active — Missing status
- **Command**: echo implicit
- **Verify**: exit_code(0)
`,
	)

	report, err := ValidateScenarios([]byte(content))
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.Runnable, 1)
	assert.Equal(t, "S1", report.Runnable[0].DisplayRef())
	assert.Equal(t, "active", report.Runnable[0].Status)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "scenario_missing_status", report.Issues[0].Code)
}

func TestValidateScenarios_NonCanonicalFieldNameIsDiagnosedAndCounted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		block     string
		wantCodes []string
		wantField string
	}{
		{
			name: "field name",
			block: `### S1: spaced-status-field — Spaced field name
- **Command**: echo ok
- **Verify**: exit_code(0)
- **Status **: active
`,
			wantCodes: []string{"scenario_missing_status", "scenario_unknown_field"},
			wantField: "Status ",
		},
		{
			name: "status value",
			block: `### S2: spaced-status-value — Spaced status value
- **Command**: echo ok
- **Verify**: exit_code(0)
- **Status**:  active
`,
			wantCodes: []string{"scenario_invalid_status"},
			wantField: "Status",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report, err := ValidateScenarios([]byte(scenarioValidationDocument(tt.block)))
			require.NoError(t, err)
			assert.Empty(t, report.Runnable)
			assert.Equal(t, 1, report.InvalidCount())
			require.Len(t, report.Issues, len(tt.wantCodes))
			codes := make([]string, 0, len(report.Issues))
			for _, issue := range report.Issues {
				codes = append(codes, issue.Code)
			}
			assert.Equal(t, tt.wantCodes, codes)
			assert.Equal(t, tt.wantField, report.Issues[len(report.Issues)-1].Field)
		})
	}
}

func scenarioValidationDocument(blocks ...string) string {
	document := `# E2E Scenarios — validation

## Project Type: CLI
## Binary: auto
## Build:

`
	for _, block := range blocks {
		document += block + "\n"
	}
	return document
}

func validScenarioBlock(ref, id, command, status, verify string) string {
	return fmt.Sprintf(`### %s: %s — Scenario
- **Command**: %s
- **Verify**: %s
- **Status**: %s
`, ref, id, command, verify, status)
}
