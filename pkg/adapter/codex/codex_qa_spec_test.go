package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodexQAWorkflowSpecAdvertisesReport keeps the Codex command description
// aligned with the other adapters. The three adapters carry independent copies of
// this string, so each needs its own guard for the set to stay consistent.
func TestCodexQAWorkflowSpecAdvertisesReport(t *testing.T) {
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
