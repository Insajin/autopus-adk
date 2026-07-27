package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncScenarios_PreservesExplicitRefsAndAssignsDisplayRef(t *testing.T) {
	t.Parallel()

	existing := &ScenarioSet{Scenarios: []Scenario{{
		Ref: "S15A", ID: "existing", Command: "auto existing", Status: "active",
	}}}
	commands := []Scenario{
		{ID: "existing", Command: "auto existing"},
		{Ref: "S-CANARY-1", ID: "canary", Command: "auto canary"},
		{ID: "new", Command: "auto new"},
	}

	updated, err := SyncScenarios(existing, commands)

	require.NoError(t, err)
	require.Len(t, updated.Scenarios, 3)
	assert.Equal(t, "S15A", updated.Scenarios[0].DisplayRef())
	assert.Equal(t, "S-CANARY-1", updated.Scenarios[1].DisplayRef())
	assert.Equal(t, "S3", updated.Scenarios[2].DisplayRef())
	assert.Equal(t, "S3", updated.Scenarios[2].Ref)
}
