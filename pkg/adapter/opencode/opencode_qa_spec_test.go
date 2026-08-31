package opencode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenCodeQAWorkflowSpecAdvertisesReport keeps the OpenCode command
// description aligned with the other adapters: the qa namespace now renders a
// visual report, and a partial subcommand list is what let that fact go missing.
func TestOpenCodeQAWorkflowSpecAdvertisesReport(t *testing.T) {
	t.Parallel()

	var found bool
	for _, spec := range workflowSpecs {
		if spec.Name != "auto-qa" {
			continue
		}
		found = true
		assert.Contains(t, spec.Description, "report")
		assert.NotContains(t, spec.Description, "init, plan, run, release, evidence")
	}
	require.True(t, found, "auto-qa workflow spec must exist")
}
