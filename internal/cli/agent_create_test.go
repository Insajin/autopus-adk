package cli

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateAgentFrontmatter_Valid verifies that a well-formed agent
// frontmatter passes validation without error.
func TestValidateAgentFrontmatter_Valid(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: my-agent\ndescription: Does something useful\n---\n")

	err := validateAgentFrontmatter(content)
	assert.NoError(t, err)
}

// TestValidateAgentFrontmatter_MissingName verifies that a missing name field
// returns an appropriate error.
func TestValidateAgentFrontmatter_MissingName(t *testing.T) {
	t.Parallel()

	content := []byte("---\ndescription: Does something useful\n---\n")

	err := validateAgentFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// TestValidateAgentFrontmatter_MissingDescription verifies that a missing
// description field returns an error.
func TestValidateAgentFrontmatter_MissingDescription(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: my-agent\n---\n")

	err := validateAgentFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

// TestValidateAgentFrontmatter_NoDelimiter verifies that content without ---
// delimiters returns an error.
func TestValidateAgentFrontmatter_NoDelimiter(t *testing.T) {
	t.Parallel()

	content := []byte("name: my-agent\ndescription: Does something\n")

	err := validateAgentFrontmatter(content)
	require.Error(t, err)
}

// TestDefaultEffortForRole verifies that known high-effort roles return "high"
// and all other roles (including unknown ones) return "medium".
func TestDefaultEffortForRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		role     string
		expected string
	}{
		// TC1: planner → high
		{name: "planner is high effort", role: "planner", expected: "high"},
		// TC2: executor → medium
		{name: "executor is medium effort", role: "executor", expected: "medium"},
		// TC3: unknown role → medium (safe fallback)
		{name: "unknown role defaults to medium", role: "new-unknown", expected: "medium"},
		// Additional high-effort roles from spec R9
		{name: "spec-writer is high effort", role: "spec-writer", expected: "high"},
		{name: "reviewer is high effort", role: "reviewer", expected: "high"},
		{name: "security-auditor is high effort", role: "security-auditor", expected: "high"},
		{name: "architect is high effort", role: "architect", expected: "high"},
		{name: "deep-worker is high effort", role: "deep-worker", expected: "high"},
		// Additional medium-effort roles
		{name: "tester is medium effort", role: "tester", expected: "medium"},
		{name: "validator is medium effort", role: "validator", expected: "medium"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := defaultEffortForRole(tt.role)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestAgentTemplate_RendersEffortField verifies that the rendered agent template
// contains the expected effort value and that other frontmatter fields remain intact.
func TestAgentTemplate_RendersEffortField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		agentName      string
		description    string
		expectedEffort string
	}{
		// TC1: planner → effort: high
		{name: "planner gets high effort", agentName: "planner", description: "Plans tasks", expectedEffort: "high"},
		// TC2: executor → effort: medium
		{name: "executor gets medium effort", agentName: "executor", description: "Executes code", expectedEffort: "medium"},
		// TC3: unknown role → effort: medium (safe fallback)
		{name: "unknown role gets medium effort", agentName: "new-unknown", description: "Some agent", expectedEffort: "medium"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := agentTemplateData{
				Name:        tt.agentName,
				Description: tt.description,
				Effort:      defaultEffortForRole(tt.agentName),
				Tools:       []string{"Read", "Write", "Bash"},
			}

			// TC4 coverage: verify that other frontmatter fields render correctly too.
			tmplParsed, err := template.New("agent").Parse(agentTemplate)
			require.NoError(t, err)

			var buf bytes.Buffer
			require.NoError(t, tmplParsed.Execute(&buf, data))

			rendered := buf.String()
			assert.Contains(t, rendered, "name: "+tt.agentName, "name field must be present")
			assert.Contains(t, rendered, "description: "+tt.description, "description field must be present")
			assert.Contains(t, rendered, "effort: "+tt.expectedEffort, "effort field must match expected value")
			assert.Contains(t, rendered, "- Read", "tools must be listed")
		})
	}
}

// TestParseTools_CommaSeparated verifies that comma-separated tool names are
// split, trimmed, and returned correctly.
func TestParseTools_CommaSeparated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single tool",
			input:    "Read",
			expected: []string{"Read"},
		},
		{
			name:     "multiple tools",
			input:    "Read,Write,Bash",
			expected: []string{"Read", "Write", "Bash"},
		},
		{
			name:     "tools with spaces",
			input:    "Read, Write, Bash",
			expected: []string{"Read", "Write", "Bash"},
		},
		{
			name:     "empty string returns defaults",
			input:    "",
			expected: []string{"Read", "Write", "Bash"},
		},
		{
			name:     "whitespace-only entries dropped",
			input:    "Read,,Bash",
			expected: []string{"Read", "Bash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseTools(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
