package omp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOMPQAWorkflowSpecAdvertisesReport pins the one line a user reads in the
// command palette. The previous description enumerated a subset of subcommands
// and silently went stale as the namespace grew, so this asserts the capability
// word rather than a fixed list.
func TestOMPQAWorkflowSpecAdvertisesReport(t *testing.T) {
	t.Parallel()

	var found bool
	for _, spec := range workflowSpecs {
		if spec.Name != "auto-qa" {
			continue
		}
		found = true
		assert.Contains(t, spec.Description, "report",
			"the qa command description must advertise the visual report surface")
		assert.NotContains(t, spec.Description, "init, plan, run, release, evidence",
			"a partial subcommand enumeration goes stale on every new subcommand")
	}
	require.True(t, found, "auto-qa workflow spec must exist")
}
