package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunPlaywright_DefaultDesktop_DoesNotFilterLegacyProjects(t *testing.T) {
	// Given / When
	run := runPlaywrightWithCapturedArgs(t, "desktop", "")

	// Then
	assert.NotContains(t, run.Args, "--project=desktop")
}

func TestRunPlaywright_OpaqueProjectSelectors_UseExactFilters(t *testing.T) {
	tests := []string{"mobile", "tablet", "Mobile Chrome", "webkit-custom"}
	for _, selector := range tests {
		selector := selector
		t.Run(selector, func(t *testing.T) {
			// Given / When
			run := runPlaywrightWithCapturedArgs(t, selector, "")

			// Then
			expected := "--project=" + selector
			assert.Contains(t, run.Args, expected)
			assert.Equal(t, 1, countExactArgument(run.Args, expected))
		})
	}
}

func TestRequiredSnapshotProjects_NoFilter_ExcludesDependencyAndTeardownSupport(t *testing.T) {
	t.Parallel()

	// Given
	proof := snapshotComparisonProof{Projects: []snapshotProjectProof{
		{Name: "main"},
		{Name: "setup", SupportOnly: true},
		{Name: "cleanup", SupportOnly: true},
	}}

	// When
	required := requiredSnapshotProjects(proof, verifyProjectSelection{NoFilter: true})

	// Then
	assert.Equal(t, []string{"main"}, required)
}
