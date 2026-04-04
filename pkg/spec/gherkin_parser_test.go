package spec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestParseGherkin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantCount    int
		wantWarnings []string
		check        func(t *testing.T, criteria []spec.Criterion)
	}{
		{
			name: "S1: three Given/When/Then scenarios parsed correctly",
			input: `### S1: User login
Given a registered user
When the user submits valid credentials
Then the system grants access

### S2: User logout
Given an authenticated user
When the user clicks logout
Then the session is terminated

### S3: Password reset
Given a user who forgot their password
When the user requests a reset
Then a reset email is sent`,
			wantCount: 3,
			check: func(t *testing.T, criteria []spec.Criterion) {
				// First scenario
				assert.Equal(t, "User login", criteria[0].Description)
				require.Len(t, criteria[0].Steps, 3)
				assert.Equal(t, "Given", criteria[0].Steps[0].Keyword)
				assert.Equal(t, "a registered user", criteria[0].Steps[0].Text)
				assert.Equal(t, "When", criteria[0].Steps[1].Keyword)
				assert.Equal(t, "Then", criteria[0].Steps[2].Keyword)

				// Second scenario
				assert.Equal(t, "User logout", criteria[1].Description)
				require.Len(t, criteria[1].Steps, 3)

				// Third scenario
				assert.Equal(t, "Password reset", criteria[2].Description)
				require.Len(t, criteria[2].Steps, 3)
			},
		},
		{
			name: "S5: Priority Must tag parsed",
			input: `### S1: Critical feature
Priority: Must
Given a system
When an event occurs
Then the system responds`,
			wantCount: 1,
			check: func(t *testing.T, criteria []spec.Criterion) {
				assert.Equal(t, "Must", criteria[0].Priority)
			},
		},
		{
			name: "S7: auto-assigned IDs AC-001 through AC-003",
			input: `### Scenario: First
Given step one
When action one
Then result one

### Scenario: Second
Given step two
When action two
Then result two

### Scenario: Third
Given step three
When action three
Then result three`,
			wantCount: 3,
			check: func(t *testing.T, criteria []spec.Criterion) {
				assert.Equal(t, "AC-001", criteria[0].ID)
				assert.Equal(t, "AC-002", criteria[1].ID)
				assert.Equal(t, "AC-003", criteria[2].ID)
			},
		},
		{
			name:         "S8: free text without Gherkin returns empty with warning",
			input:        "This is just plain text without any scenarios or keywords.",
			wantCount:    0,
			wantWarnings: []string{"no Gherkin scenarios found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			criteria, warnings := spec.ParseGherkin(tt.input)

			assert.Len(t, criteria, tt.wantCount)

			if tt.wantWarnings != nil {
				assert.Equal(t, tt.wantWarnings, warnings)
			}

			if tt.check != nil {
				tt.check(t, criteria)
			}
		})
	}
}
